package wallet

import (
	"context"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/ledgerentry"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
)

func (s *Service) AdjustLedger(ctx context.Context, actorID, original, delta, reason, key string) error {
	if strings.TrimSpace(reason) == "" || app.ValidateKey(key) != nil {
		return app.Invalid("请提供原因与幂等键")
	}

	amount, err := money.ParseFen(strings.TrimPrefix(delta, "-"))
	if err != nil || amount.IsZero() {
		return app.Invalid("调整金额必须为非零整数分")
	}

	if strings.HasPrefix(delta, "-") {
		amount = amount.Neg()
	}

	originalEntry, err := s.Client.LedgerEntry.Get(ctx, original)
	if err != nil {
		return app.WrapMissing(err)
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		userWallet, err := tx.Wallet.Query().
			Where(wallet.UserIDEQ(originalEntry.UserID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, err
		}

		businessKey := "adjust:" + key
		old, err := tx.LedgerEntry.Query().
			Where(ledgerentry.BusinessKeyEQ(businessKey)).
			Only(ctx)
		if err == nil {
			if old.OriginalID != original || !old.AmountFen.Equal(amount) || old.Reason != reason {
				return false, app.Conflict("幂等键与原调整请求不一致")
			}

			return true, nil
		}

		if !ent.IsNotFound(err) {
			return false, err
		}

		if userWallet.AvailableFen.Add(amount).
			IsNegative() {
			return false, app.Conflict("调整后余额不足，需先核对已提现资金")
		}

		adjustmentEntry, err := tx.LedgerEntry.Create().
			SetUserID(originalEntry.UserID).
			SetBusinessKey(businessKey).
			SetKind(enum.LedgerAdjustment).
			SetAmountFen(amount).
			SetReferenceID(original).
			SetOriginalID(original).
			SetReason(reason).
			Save(ctx)
		if err != nil {
			return false, err
		}

		if _, err = tx.Wallet.UpdateOneID(userWallet.ID).
			SetAvailableFen(userWallet.AvailableFen.Add(amount)).
			Save(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditLedgerAdjust, adjustmentEntry.ID, reason, "原流水："+original, "调整金额："+amount.String()+" 分")
	})
	return err
}
