package wallet

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/shopspring/decimal"
)

func (s *Service) ApplyTransferResult(ctx context.Context, id string, status enum.WithdrawalStatus, providerID string) error {
	if status != enum.WithdrawalSuccess && status != enum.WithdrawalFailed && status != enum.WithdrawalCancelled {
		return app.Invalid("非转账终态")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		withdrawalRecord, err := tx.Withdrawal.Query().
			Where(withdrawal.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, err
		}

		if providerID == "" || withdrawalRecord.ProviderID != "" && withdrawalRecord.ProviderID != providerID {
			return false, app.Conflict("微信转账单号不匹配，需要核对")
		}

		if withdrawalRecord.Status == enum.WithdrawalSuccess || withdrawalRecord.Status == enum.WithdrawalFailed || withdrawalRecord.Status == enum.WithdrawalCancelled {
			if withdrawalRecord.Status == status {
				return true, nil
			}

			return false, app.Conflict("转账终态不允许逆转")
		}

		userWallet, err := tx.Wallet.Query().
			Where(wallet.UserIDEQ(withdrawalRecord.UserID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, err
		}

		if userWallet.FrozenFen.LessThan(withdrawalRecord.AmountFen) {
			return false, app.Conflict("冻结余额异常，需要核对")
		}

		walletUpdate := tx.Wallet.UpdateOneID(userWallet.ID).
			SetFrozenFen(userWallet.FrozenFen.Sub(withdrawalRecord.AmountFen))
		kind := enum.LedgerWithdrawalSuccess
		amount := decimal.Zero
		if status == enum.WithdrawalSuccess {
			walletUpdate = walletUpdate.SetWithdrawnFen(userWallet.WithdrawnFen.Add(withdrawalRecord.AmountFen))
		} else {
			walletUpdate = walletUpdate.SetAvailableFen(userWallet.AvailableFen.Add(withdrawalRecord.AmountFen))
			kind = enum.LedgerWithdrawalRefund
			amount = withdrawalRecord.AmountFen
		}

		if _, err = walletUpdate.Save(ctx); err != nil {
			return false, err
		}

		if _, err = tx.LedgerEntry.Create().
			SetUserID(withdrawalRecord.UserID).
			SetBusinessKey("transfer-terminal:" + id).
			SetKind(kind).
			SetAmountFen(amount).
			SetReferenceID(id).
			Save(ctx); err != nil {
			return false, err
		}

		if _, err = tx.Withdrawal.UpdateOneID(id).
			SetStatus(status).
			SetReason("").
			SetProviderID(providerID).
			Save(ctx); err != nil {
			return false, err
		}

		reason := "微信转账失败"
		if status == enum.WithdrawalCancelled {
			reason = "微信转账已取消"
		}

		card := wxnotice.Transfer(withdrawalRecord.AmountFen.Shift(-2).StringFixed(2), withdrawalRecord.CreatedAt, status == enum.WithdrawalSuccess, reason)
		return true, s.Notify(ctx, tx, withdrawalRecord.UserID, "提现结果", string(status), id, card)
	})
	return err
}
