package wallet

import (
	"context"
	dbsql "database/sql"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/creativeearning"
	"baituber.cn/yanxuan/v2/ent/ledgerentry"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
	"github.com/shopspring/decimal"
)

func (s *Service) Wallet(ctx context.Context, userID string) (WalletDTO, error) {
	tx, err := s.Client.BeginTx(ctx, &dbsql.TxOptions{
		Isolation: dbsql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return WalletDTO{}, err
	}

	defer tx.Rollback()
	userWallet, err := tx.Wallet.Query().
		Where(wallet.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return WalletDTO{}, app.WrapMissing(err)
	}

	now := app.BeijingTime(s.Now())
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	credits := tx.LedgerEntry.Query().
		Where(ledgerentry.UserIDEQ(userID), ledgerentry.KindEQ(enum.LedgerSettlementCredit))
	earned, err := sumWalletAmount(ctx, credits.Clone().Aggregate(ent.As(ent.Sum(ledgerentry.FieldAmountFen), "amount")))
	if err != nil {
		return WalletDTO{}, err
	}

	monthEarned, err := sumWalletAmount(ctx, credits.
		Where(ledgerentry.CreatedAtGTE(month), ledgerentry.CreatedAtLT(month.AddDate(0, 1, 0))).
		Aggregate(ent.As(ent.Sum(ledgerentry.FieldAmountFen), "amount")))
	if err != nil {
		return WalletDTO{}, err
	}

	pending, err := sumWalletAmount(ctx, tx.Submission.Query().
		Where(
			submission.StatusIn(enum.SubmissionApproved, enum.SubmissionWaived),
			submission.SettlementIDEQ(""),
		).
		Aggregate(func(selector *sql.Selector) string {
			assignments := sql.Table(assignment.Table)
			selector.Join(assignments).On(selector.C(submission.FieldAssignmentID), assignments.C(assignment.FieldID))
			selector.Where(sql.EQ(assignments.C(assignment.FieldUserID), userID))
			return sql.As(sql.Sum(assignments.C(assignment.FieldPriceFen)), "amount")
		}))
	if err != nil {
		return WalletDTO{}, err
	}

	creativePending, err := sumWalletAmount(ctx, tx.CreativeEarning.Query().Where(creativeearning.UserIDEQ(userID), creativeearning.BatchIDEQ("")).Aggregate(ent.As(ent.Sum(creativeearning.FieldAmountFen), "amount")))
	if err != nil {
		return WalletDTO{}, err
	}

	pending = decimal.RequireFromString(pending).Add(decimal.RequireFromString(creativePending)).String()
	if err := tx.Commit(); err != nil {
		return WalletDTO{}, err
	}

	return WalletDTO{
		AvailableFen:     userWallet.AvailableFen.String(),
		FrozenFen:        userWallet.FrozenFen.String(),
		WithdrawnFen:     userWallet.WithdrawnFen.String(),
		EarnedFen:        earned,
		MonthEarnedFen:   monthEarned,
		PendingFen:       pending,
		MinWithdrawalFen: minWithdrawalFen,
		MaxWithdrawalFen: maxWithdrawalFen,
	}, nil
}

func sumWalletAmount(ctx context.Context, query interface {
	Scan(context.Context, any) error
}) (string, error) {
	var rows []struct {
		Amount decimal.NullDecimal `json:"amount"`
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return "", err
	}

	if len(rows) == 0 || !rows[0].Amount.Valid {
		return "0", nil
	}

	return rows[0].Amount.Decimal.String(), nil
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

func (s *Service) WithdrawalsInMonth(ctx context.Context, month string) ([]WithdrawalDTO, error) {
	start, err := time.ParseInLocation("2006-01", month, app.BeijingTime(s.Now()).Location())
	if err != nil || start.Format("2006-01") != month {
		return nil, app.Invalid("提现月份无效")
	}

	end := start.AddDate(0, 1, 0)
	rows, err := s.Client.Withdrawal.Query().
		Where(withdrawal.Or(
			withdrawal.And(
				withdrawal.StatusEQ(enum.WithdrawalSuccess),
				withdrawal.UpdatedAtGTE(start),
				withdrawal.UpdatedAtLT(end),
			),
			withdrawal.And(
				withdrawal.StatusNEQ(enum.WithdrawalSuccess),
				withdrawal.CreatedAtGTE(start),
				withdrawal.CreatedAtLT(end),
			),
		)).
		Order(ent.Desc(withdrawal.FieldCreatedAt, withdrawal.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]WithdrawalDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.withdrawalDTO(row))
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
