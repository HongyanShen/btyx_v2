package wallet

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/ledgerentry"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Wallet(ctx context.Context, userID string) (WalletDTO, error) {
	userWallet, err := s.Client.Wallet.Query().
		Where(wallet.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return WalletDTO{}, app.WrapMissing(err)
	}

	return WalletDTO{
		AvailableFen: userWallet.AvailableFen.String(),
		FrozenFen:    userWallet.FrozenFen.String(),
		WithdrawnFen: userWallet.WithdrawnFen.String(),
	}, nil
}

func (s *Service) withdrawalDTO(record *ent.Withdrawal) WithdrawalDTO {
	return WithdrawalDTO{
		ID:          record.ID,
		UserID:      record.UserID,
		AmountFen:   record.AmountFen.String(),
		Status:      record.Status,
		Reason:      record.Reason,
		PackageInfo: record.PackageInfo,
		MerchantID:  s.Config.Payment.MerchantID,
		CreatedAt:   record.CreatedAt,
	}
}

func (s *Service) Withdrawals(ctx context.Context, userID string, role enum.Role) ([]WithdrawalDTO, error) {
	query := s.Client.Withdrawal.Query()
	if role != enum.RoleAdmin {
		query = query.Where(withdrawal.UserIDEQ(userID))
	}

	rows, err := query.Order(ent.Desc(withdrawal.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := []WithdrawalDTO{}
	for _, withdrawalRecord := range rows {
		out = append(out, s.withdrawalDTO(withdrawalRecord))
	}

	return out, nil
}

func (s *Service) Ledger(ctx context.Context, userID string, role enum.Role) ([]LedgerDTO, error) {
	query := s.Client.LedgerEntry.Query()
	if role != enum.RoleAdmin {
		query = query.Where(ledgerentry.UserIDEQ(userID))
	}

	rows, err := query.Order(ent.Desc(ledgerentry.FieldCreatedAt)).
		Limit(300).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := []LedgerDTO{}
	for _, entry := range rows {
		out = append(out, LedgerDTO{
			ID:          entry.ID,
			UserID:      entry.UserID,
			Kind:        entry.Kind,
			AmountFen:   entry.AmountFen.String(),
			ReferenceID: entry.ReferenceID,
			OriginalID:  entry.OriginalID,
			Reason:      entry.Reason,
			CreatedAt:   entry.CreatedAt,
		})
	}

	return out, nil
}
