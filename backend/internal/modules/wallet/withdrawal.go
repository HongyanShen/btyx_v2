package wallet

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/shopspring/decimal"
)

func (s *Service) Withdraw(ctx context.Context, userID, amountText, key string) (WithdrawalDTO, error) {
	if err := app.ValidateKey(key); err != nil {
		return WithdrawalDTO{}, err
	}

	amount, err := money.ParseFen(amountText)
	if err != nil || amount.LessThan(decimal.NewFromInt(10)) || amount.GreaterThan(decimal.NewFromInt(20000)) {
		return WithdrawalDTO{}, app.Invalid("本地业务配置：单笔提现0.10至200.00元")
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (WithdrawalDTO, error) {
		userWallet, err := tx.Wallet.Query().
			Where(wallet.UserIDEQ(userID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return WithdrawalDTO{}, app.WrapMissing(err)
		}

		requestKey := userID + ":" + key
		old, err := tx.Withdrawal.Query().
			Where(withdrawal.RequestKeyEQ(requestKey)).
			Only(ctx)
		if err == nil {
			if !old.AmountFen.Equal(amount) {
				return WithdrawalDTO{}, app.Conflict("幂等键与原提现金额不一致")
			}

			return s.withdrawalDTO(old), nil
		}

		if !ent.IsNotFound(err) {
			return WithdrawalDTO{}, err
		}

		if userWallet.AvailableFen.LessThan(amount) {
			return WithdrawalDTO{}, app.Conflict("可提现余额不足")
		}

		withdrawalRecord, err := tx.Withdrawal.Create().
			SetUserID(userID).
			SetRequestKey(requestKey).
			SetAmountFen(amount).
			Save(ctx)
		if err != nil {
			return WithdrawalDTO{}, err
		}

		if _, err = tx.Wallet.UpdateOneID(userWallet.ID).
			SetAvailableFen(userWallet.AvailableFen.Sub(amount)).
			SetFrozenFen(userWallet.FrozenFen.Add(amount)).
			Save(ctx); err != nil {
			return WithdrawalDTO{}, err
		}

		if _, err = tx.LedgerEntry.Create().
			SetUserID(userID).
			SetBusinessKey("freeze:" + withdrawalRecord.ID).
			SetKind(enum.LedgerWithdrawalFreeze).
			SetAmountFen(amount.Neg()).
			SetReferenceID(withdrawalRecord.ID).
			Save(ctx); err != nil {
			return WithdrawalDTO{}, err
		}

		if _, err = tx.Job.Create().
			SetKind(enum.JobTransfer).
			SetNextAt(s.Now()).
			SetReferenceID(withdrawalRecord.ID).
			SetRequestID(logging.RequestID(ctx)).
			Save(ctx); err != nil {
			return WithdrawalDTO{}, err
		}

		return s.withdrawalDTO(withdrawalRecord), app.Audit(ctx, tx, userID, enum.AuditWithdrawalRequest, withdrawalRecord.ID, "")
	})
}
