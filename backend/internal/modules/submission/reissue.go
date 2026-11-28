package submission

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/ent/user"
	agreements "baituber.cn/yanxuan/v2/internal/modules/agreement"
	tasks "baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Reissue(ctx context.Context, actorID, id string, platform enum.Platform, reason string, keepPay bool) (tasks.AssignmentDTO, error) {
	if !platform.Valid() {
		return tasks.AssignmentDTO{}, app.Invalid("发布平台无效")
	}

	if strings.TrimSpace(reason) == "" {
		return tasks.AssignmentDTO{}, app.Invalid("请填写违规或重派原因")
	}

	requestKey := "reissue:" + id + ":" + string(platform)
	original, err := s.Client.Assignment.Get(ctx, id)
	if err != nil {
		return tasks.AssignmentDTO{}, app.WrapMissing(err)
	}

	newID, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		var taskRecord *ent.Task
		var dispatchErr error
		if !keepPay {
			// Keep the same lock order as other dispatch operations.
			_, dispatchErr = tx.User.Query().
				Where(user.IDEQ(original.UserID)).
				ForUpdate().
				Only(ctx)
			if dispatchErr == nil {
				taskRecord, dispatchErr = tx.Task.Query().
					Where(task.IDEQ(original.TaskID)).
					ForUpdate().
					Only(ctx)
			}

			if dispatchErr != nil && !ent.IsNotFound(dispatchErr) {
				return "", dispatchErr
			}
		}

		assignmentRecord, err := tx.Assignment.Query().
			Where(assignment.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return "", app.WrapMissing(err)
		}

		old, err := tx.Assignment.Query().
			Where(assignment.RequestKeyEQ(requestKey)).
			Only(ctx)
		if err == nil {
			return old.ID, nil
		}

		if !ent.IsNotFound(err) {
			return "", err
		}

		if dispatchErr != nil {
			return "", app.WrapMissing(dispatchErr)
		}

		platformSubmission, err := tx.Submission.Query().
			Where(submission.AssignmentIDEQ(id), submission.PlatformEQ(platform)).
			Only(ctx)
		if err != nil {
			return "", app.WrapMissing(err)
		}

		if platformSubmission.SettlementID != "" {
			return "", app.Conflict("已入账，请走关联原流水的调整流程")
		}

		if platformSubmission.Status != enum.SubmissionApproved && platformSubmission.Status != enum.SubmissionWaived {
			return "", app.Conflict("仅已通过的平台可标记")
		}

		if keepPay {
			_, err = tx.Submission.UpdateOneID(platformSubmission.ID).
				SetStatus(enum.SubmissionWaived).
				SetReason(reason).
				SetReviewerID(actorID).
				Save(ctx)
			if err != nil {
				return "", err
			}

			return id, app.Audit(ctx, tx, actorID, enum.AuditSubmissionWaive, platformSubmission.ID, reason,
				"平台："+string(platform), app.AuditFlag("照常结算", keepPay))
		}

		if err = agreements.CheckEligibility(ctx, tx, assignmentRecord.UserID); err != nil {
			return "", err
		}

		version, err := tx.TaskVersion.Get(ctx, taskRecord.CurrentVersionID)
		if err != nil {
			return "", err
		}

		policy, err := tasks.LoadMarketingPolicy(ctx, tx.Client())
		if err != nil {
			return "", err
		}

		_, err = tx.Submission.UpdateOneID(platformSubmission.ID).
			SetStatus(enum.SubmissionInvalid).
			SetReason(reason).
			Save(ctx)
		if err != nil {
			return "", err
		}

		allocatedAt := s.Now()
		day := app.BusinessDay(allocatedAt)
		quotaKey, err := tasks.DailyQuotaSlot(ctx, tx, assignmentRecord.UserID, day, policy.DailyLimit)
		if err != nil {
			return "", err
		}

		createAssignment := tx.Assignment.Create().
			SetUserID(assignmentRecord.UserID).
			SetTaskID(assignmentRecord.TaskID).
			SetVersionID(version.ID).
			SetPriceFen(policy.AllocationPrice(version.PriceFen)).
			SetPlatforms([]enum.Platform{platform}).
			SetSource(enum.SourceReissued).
			SetSourceID(id).
			SetBusinessDate(day).
			SetCreatedAt(allocatedAt).
			SetDueDate(day).
			SetExpiresAt(policy.Deadline(allocatedAt)).
			SetRequestKey(requestKey).
			SetActorID(actorID)
		if quotaKey != "" {
			createAssignment.SetQuotaKey(quotaKey)
		}

		newAssignment, err := createAssignment.Save(ctx)
		if err != nil {
			return "", err
		}

		if err = tasks.RefreshAssignment(ctx, tx, id); err != nil {
			return "", err
		}

		card := wxnotice.Assignment(version.Title, "重新派发 · "+wxnotice.Platform(platform), reason, newAssignment.ID, *newAssignment.ExpiresAt)
		if err = s.Notify(ctx, tx, assignmentRecord.UserID, "任务重新派发", reason, newAssignment.ID, card); err != nil {
			return "", err
		}

		return newAssignment.ID, app.Audit(ctx, tx, actorID, enum.AuditTaskReissue, newAssignment.ID, reason,
			"平台："+string(platform), app.AuditFlag("照常结算", keepPay))
	})
	if err != nil {
		return tasks.AssignmentDTO{}, err
	}

	return s.TaskQueries.AdminAssignment(ctx, newID)
}
