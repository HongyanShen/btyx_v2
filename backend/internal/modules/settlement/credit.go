package settlement

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"
	"database/sql"
	"errors"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/settlementbatch"
	"baituber.cn/yanxuan/v2/ent/wallet"
	tasks "baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/shopspring/decimal"
)

func (s *Service) Settle(ctx context.Context, actorID string, input SettlementInput) (BatchDTO, error) {
	if err := app.ValidateKey(input.Key); err != nil {
		return BatchDTO{}, err
	}

	if !app.ValidDate(input.Through) || input.Through > s.Day() {
		return BatchDTO{}, app.Invalid("只能结算已到期的记录")
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (BatchDTO, error) {
		old, err := existingBatch(ctx, tx, input)
		if err == nil {
			return batchDTO(old), nil
		}

		if !ent.IsNotFound(err) {
			return BatchDTO{}, err
		}

		now := s.Now()
		exactPeriod := input.ReviewToken != ""
		preview, err := settlementPreview(ctx, tx.Client(), input.Through, now, exactPeriod)
		if err != nil {
			return BatchDTO{}, err
		}

		if preview.Count == 0 {
			// Another request may have committed between the first lookup and preview.
			old, err := existingBatch(ctx, tx, input)
			if err == nil {
				return batchDTO(old), nil
			}

			if !ent.IsNotFound(err) {
				return BatchDTO{}, err
			}

			return BatchDTO{}, app.Conflict("没有符合条件的待结算记录（需审核通过、播放数据齐全且到期）")
		}

		assignmentIDs := make([]string, 0, len(preview.Lines))
		userIDs := make([]string, 0, len(preview.Lines))
		for _, line := range preview.Lines {
			assignmentIDs = append(assignmentIDs, line.AssignmentID)
			userIDs = append(userIDs, line.UserID)
		}

		// Lock every assignment before any wallet, in a stable order across batches.
		if _, err := tx.Assignment.Query().
			Where(assignment.IDIn(assignmentIDs...)).
			Order(ent.Asc(assignment.FieldID)).
			ForUpdate().
			All(ctx); err != nil {
			return BatchDTO{}, err
		}

		if err := lockCreative(ctx, tx, preview.Lines); err != nil {
			return BatchDTO{}, err
		}

		if exactPeriod {
			current, err := settlementPreview(ctx, tx.Client(), input.Through, s.Now(), true)
			if err != nil {
				return BatchDTO{}, err
			}

			if previewToken(current) != input.ReviewToken {
				old, err := existingBatch(ctx, tx, input)
				if err == nil {
					return batchDTO(old), nil
				}

				if !ent.IsNotFound(err) {
					return BatchDTO{}, err
				}

				return BatchDTO{}, app.Conflict("核对期间记录已变化，请重新核对")
			}

			// The records being credited must also be the records we locked above.
			if previewToken(preview) != input.ReviewToken {
				return BatchDTO{}, app.Conflict("核对期间记录已变化，请重新核对")
			}
		}

		if _, err := tx.Wallet.Query().
			Where(wallet.UserIDIn(userIDs...)).
			Order(ent.Asc(wallet.FieldID)).
			ForUpdate().
			All(ctx); err != nil {
			return BatchDTO{}, err
		}

		amount, _ := decimal.NewFromString(preview.AmountFen)
		batchID := app.NewID()
		err = tx.SettlementBatch.Create().
			SetID(batchID).
			SetRequestKey(input.Key).
			SetThrough(input.Through).
			SetCreatorID(actorID).
			SetAmountFen(amount).
			SetCount(preview.Count).
			OnConflictColumns(settlementbatch.FieldRequestKey).
			DoNothing().
			Exec(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return BatchDTO{}, err
		}

		batch, err := existingBatch(ctx, tx, input)
		if err != nil {
			return BatchDTO{}, err
		}

		if batch.ID != batchID {
			return batchDTO(batch), nil
		}

		credited := make(map[string]decimal.Decimal)
		for _, line := range preview.Lines {
			if line.Creative {
				amount, err := creditCreative(ctx, tx, batch.ID, line)
				if err != nil {
					return BatchDTO{}, err
				}

				credited[line.UserID] = credited[line.UserID].Add(amount)
				continue
			}

			platformSubmission, err := tx.Submission.Get(ctx, line.SubmissionID)
			if err != nil {
				return BatchDTO{}, err
			}

			if platformSubmission.SettlementID != "" || platformSubmission.Status != enum.SubmissionApproved && platformSubmission.Status != enum.SubmissionWaived {
				return BatchDTO{}, app.Conflict("核对期间记录已变化，请重新核对")
			}

			if platformSubmission.Screenshot == "" || platformSubmission.EligibleDate == "" || platformSubmission.EligibleDate > input.Through ||
				(exactPeriod && platformSubmission.EligibleDate != input.Through) ||
				(platformSubmission.SettlementEligibleAt != nil && platformSubmission.SettlementEligibleAt.After(now)) ||
				platformSubmission.Visibility != enum.VisibilityVisible && platformSubmission.Visibility != enum.VisibilityManualVerified {
				return BatchDTO{}, app.Conflict("核对期间结算条件已变化，请重新核对")
			}

			amount, _ := decimal.NewFromString(line.AmountFen)
			settlementLine, err := tx.SettlementLine.Create().
				SetBatchID(batch.ID).
				SetSubmissionID(platformSubmission.ID).
				SetAssignmentID(line.AssignmentID).
				SetUserID(line.UserID).
				SetAmountFen(amount).
				SetPlatform(line.Platform).
				Save(ctx)
			if err != nil {
				return BatchDTO{}, err
			}

			userWallet, err := tx.Wallet.Query().
				Where(wallet.UserIDEQ(line.UserID)).
				Only(ctx)
			if err != nil {
				return BatchDTO{}, err
			}

			if _, err = tx.Wallet.UpdateOneID(userWallet.ID).
				SetAvailableFen(userWallet.AvailableFen.Add(amount)).
				Save(ctx); err != nil {
				return BatchDTO{}, err
			}

			if _, err = tx.LedgerEntry.Create().
				SetUserID(line.UserID).
				SetBusinessKey("settlement:" + platformSubmission.ID).
				SetKind(enum.LedgerSettlementCredit).
				SetAmountFen(amount).
				SetReferenceID(settlementLine.ID).
				Save(ctx); err != nil {
				return BatchDTO{}, err
			}

			if _, err = tx.Submission.UpdateOneID(platformSubmission.ID).
				SetSettlementID(batch.ID).
				Save(ctx); err != nil {
				return BatchDTO{}, err
			}

			if err = tasks.RefreshAssignment(ctx, tx, line.AssignmentID); err != nil {
				return BatchDTO{}, err
			}

			credited[line.UserID] = credited[line.UserID].Add(amount)
		}

		for userID, amount := range credited {
			message := "本次分发结算奖励 " + amount.Div(decimal.NewFromInt(100)).StringFixed(2) + " 元已进入可提现余额"
			if err := s.Notify(ctx, tx, userID, "分发结算已入账", message, batch.ID, wxnotice.Credit(amount.Shift(-2).StringFixed(2))); err != nil {
				return BatchDTO{}, err
			}
		}

		return batchDTO(batch), app.Audit(ctx, tx, actorID, enum.AuditSettlementCredit, batch.ID, "",
			"结算截止日："+batch.Through, "入账金额："+batch.AmountFen.String()+" 分")
	})
}
