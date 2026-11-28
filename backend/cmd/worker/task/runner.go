package task

import (
	"context"
	"errors"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/job"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
)

type Runner struct {
	services *business.Service
	gateway  transfer.Gateway
}

func New(services *business.Service, gateway transfer.Gateway) *Runner {
	return &Runner{
		services: services,
		gateway:  gateway,
	}
}

func (runner *Runner) Tick(ctx context.Context) error {
	if err := runner.services.ExpireMarketingAssignments(ctx); err != nil {
		return err
	}

	eligible := job.StatusIn(enum.JobQueued, enum.JobRetry, enum.JobRunning)
	if runner.gateway != nil {
		eligible = job.Or(eligible, job.And(job.StatusEQ(enum.JobBlockedConfig), job.KindEQ(enum.JobTransfer)))
	}

	jobs, err := runner.services.Client.Job.Query().
		Where(
			job.KindEQ(enum.JobTransfer),
			eligible,
			job.NextAtLTE(runner.services.Now()),
		).
		Order(ent.Asc(job.FieldCreatedAt)).
		Limit(20).
		All(ctx)
	if err != nil {
		return err
	}

	for _, candidate := range jobs {
		if err := runner.runCandidate(ctx, candidate); err != nil {
			return err
		}
	}

	return nil
}

func (runner *Runner) runCandidate(ctx context.Context, candidate *ent.Job) (err error) {
	requestID := candidate.RequestID
	if requestID == "" {
		requestID = candidate.ID
	}

	jobContext, span := telemetry.StartJob(ctx, candidate.TraceParent, "wechat.transfer", int(candidate.Attempts+1))
	defer func() { telemetry.Finish(span, err) }()
	jobContext = logging.WithRequestID(jobContext, requestID)
	claimed, err := runner.claim(jobContext, candidate.ID)
	if err != nil {
		logging.Error(jobContext, err).Str("job_id", candidate.ID).Msg("background job claim failed")

		return err
	}

	if claimed == nil {
		return nil
	}

	logging.Info(jobContext).
		Str("job_id", claimed.ID).
		Str("job_kind", string(claimed.Kind)).
		Int64("attempt", claimed.Attempts).
		Msg("background job started")

	if err := runner.process(jobContext, claimed); err != nil {
		logging.Error(jobContext, err).Str("job_id", claimed.ID).Msg("background job failed")

		return err
	}

	return nil
}

func (runner *Runner) claim(ctx context.Context, id string) (*ent.Job, error) {
	return app.Transaction(ctx, runner.services.Runtime, func(tx *ent.Tx) (*ent.Job, error) {
		record, err := tx.Job.Query().
			Where(job.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return nil, err
		}

		if !runner.canClaim(record) {
			return nil, nil
		}

		if record.NextAt.After(runner.services.Now()) {
			return nil, nil
		}

		return tx.Job.UpdateOneID(id).
			SetStatus(enum.JobRunning).
			SetAttempts(record.Attempts + 1).
			SetNextAt(runner.services.Now().Add(time.Minute)).
			Save(ctx)
	})
}

func (runner *Runner) canClaim(record *ent.Job) bool {
	if record.Status == enum.JobBlockedConfig {
		return runner.gateway != nil && record.Kind == enum.JobTransfer
	}

	return record.Status == enum.JobQueued || record.Status == enum.JobRetry || record.Status == enum.JobRunning
}

func (runner *Runner) process(ctx context.Context, record *ent.Job) error {

	withdrawal, err := runner.services.Client.Withdrawal.Get(ctx, record.ReferenceID)
	if err != nil {
		return err
	}

	telemetry.SetUser(ctx, withdrawal.UserID, enum.RoleUser)
	if terminal(withdrawal.Status) {
		return runner.finishJob(ctx, record.ID, enum.JobDone, "", 0)
	}

	if runner.gateway == nil {
		_, err := runner.pendingUpdate(withdrawal.ID).
			SetStatus(enum.WithdrawalBlockedConfig).
			SetReason("微信提现通道尚未配置，资金保持冻结等待处理").
			Save(ctx)
		if err != nil {
			return err
		}

		return runner.finishJob(ctx, record.ID, enum.JobBlockedConfig, "微信提现通道尚未配置", 0)
	}

	result, err := runner.callGateway(ctx, record, withdrawal)
	if err != nil {
		logging.Error(ctx, err).
			Str("job_id", record.ID).
			Str("withdrawal_id", withdrawal.ID).
			Msg("transfer result unconfirmed; query original bill on retry")

		_, updateError := runner.pendingUpdate(withdrawal.ID).
			SetStatus(enum.WithdrawalUnknown).
			SetReason("转账结果待确认，系统将自动查单").
			Save(ctx)
		if updateError != nil {
			return updateError
		}

		return runner.finishJob(ctx, record.ID, enum.JobRetry, "转账结果待确认", 30*time.Second)
	}

	openID, err := runner.services.WechatOpenID(ctx, withdrawal.UserID)
	if err != nil {
		return err
	}

	if result.AmountFen != withdrawal.AmountFen.String() || result.OpenID != openID {
		return runner.finishJob(ctx, record.ID, enum.JobRetry, "转账金额或收款身份不匹配，需要核对", 30*time.Second)
	}

	if withdrawal.ProviderID != "" && withdrawal.ProviderID != result.ProviderID {
		return runner.finishJob(ctx, record.ID, enum.JobRetry, "微信转账单号不匹配，需要核对", 30*time.Second)
	}

	if terminal(result.Status) {
		if err := runner.services.ApplyTransferResult(ctx, withdrawal.ID, result.Status, result.ProviderID); err != nil {
			return err
		}

		return runner.finishJob(ctx, record.ID, enum.JobDone, "", 0)
	}

	update := runner.pendingUpdate(withdrawal.ID).
		SetStatus(result.Status).
		SetReason("").
		SetProviderID(result.ProviderID)
	if result.PackageInfo != "" {
		update.SetPackageInfo(result.PackageInfo)
	}

	_, err = update.Save(ctx)
	if err != nil {
		return err
	}

	return runner.finishJob(ctx, record.ID, enum.JobRetry, "", 30*time.Second)
}

func (runner *Runner) pendingUpdate(id string) *ent.WithdrawalUpdate {
	return runner.services.Client.Withdrawal.Update().Where(
		withdrawal.IDEQ(id),
		withdrawal.StatusNotIn(enum.WithdrawalSuccess, enum.WithdrawalFailed, enum.WithdrawalCancelled),
	)
}

func (runner *Runner) callGateway(ctx context.Context, record *ent.Job, withdrawal *ent.Withdrawal) (transfer.Result, error) {
	if withdrawal.ProviderID != "" {
		return runner.gateway.Query(ctx, withdrawal.ID)
	}

	if record.Attempts > 1 && withdrawal.Status != enum.WithdrawalBlockedConfig {
		result, err := runner.gateway.Query(ctx, withdrawal.ID)
		if !errors.Is(err, transfer.ErrNotFound) {
			return result, err
		}

		// Never recreate old bills outside a bounded recovery window. A provider
		// NOT_FOUND after its query retention window cannot prove non-payment.
		if runner.services.Now().Sub(withdrawal.CreatedAt) > 24*time.Hour {
			return transfer.Result{}, err
		}
	}

	_, err := runner.pendingUpdate(withdrawal.ID).
		SetStatus(enum.WithdrawalSubmitting).
		SetReason("").
		Save(ctx)
	if err != nil {
		return transfer.Result{}, err
	}

	openID, err := runner.services.WechatOpenID(ctx, withdrawal.UserID)
	if err != nil {
		return transfer.Result{}, err
	}

	return runner.gateway.Submit(ctx, transfer.Request{
		ID:        withdrawal.ID,
		OpenID:    openID,
		AmountFen: withdrawal.AmountFen.String(),
	})
}

func (runner *Runner) finishJob(ctx context.Context, id string, status enum.JobStatus, reason string, delay time.Duration) error {
	_, err := runner.services.Client.Job.UpdateOneID(id).
		SetStatus(status).
		SetLastError(reason).
		SetNextAt(runner.services.Now().Add(delay)).
		Save(ctx)
	if err == nil {
		logging.Info(ctx).
			Str("job_id", id).
			Str("job_status", string(status)).
			Str("retry_after", delay.String()).
			Msg("background job updated")
	}

	return err
}

func terminal(status enum.WithdrawalStatus) bool {
	return status == enum.WithdrawalSuccess || status == enum.WithdrawalFailed || status == enum.WithdrawalCancelled
}
