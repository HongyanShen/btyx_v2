package settlement

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/creativeearning"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"context"
	"github.com/shopspring/decimal"
	"time"
)

func creativeRevision(row *ent.CreativeEarning) string {
	return "creative:" + row.ID + ":" + row.AmountFen.String()
}

func pendingCreative(ctx context.Context, client *ent.Client, through string, now time.Time, exact bool) ([]*ent.CreativeEarning, error) {
	day, err := time.ParseInLocation(time.DateOnly, through, app.BeijingTime(now).Location())
	if err != nil {
		return nil, err
	}

	query := client.CreativeEarning.Query().
		Where(creativeearning.BatchIDEQ(""), creativeearning.AmountFenGT(decimal.Zero), creativeearning.EligibleAtLTE(now), creativeearning.EligibleAtLT(day.AddDate(0, 0, 1)))
	if exact {
		query.Where(creativeearning.EligibleAtGTE(day))
	}

	return query.Order(ent.Asc(creativeearning.FieldID)).Limit(1001).All(ctx)
}

func lockCreative(ctx context.Context, tx *ent.Tx, lines []SettlementPreviewLine) error {
	var ids []string
	for _, line := range lines {
		if line.Creative {
			ids = append(ids, line.SubmissionID)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	_, err := tx.CreativeEarning.Query().
		Where(creativeearning.IDIn(ids...)).
		Order(ent.Asc(creativeearning.FieldID)).
		ForUpdate().
		All(ctx)
	return err
}

func creativeKind(row *ent.CreativeEarning) string {
	if row.LikesReward {
		return "创作任务 · 点赞奖励"
	}

	return "创作任务 · 创作费 + 发布费"
}

func creditCreative(ctx context.Context, tx *ent.Tx, batch string, line SettlementPreviewLine) (decimal.Decimal, error) {
	row, err := tx.CreativeEarning.Get(ctx, line.SubmissionID)
	if err != nil {
		return decimal.Zero, err
	}

	if row.BatchID != "" || row.AmountFen.String() != line.AmountFen {
		return decimal.Zero, app.Conflict("创作奖励已经变化，请重新核对")
	}

	account, err := tx.Wallet.Query().Where(wallet.UserIDEQ(row.UserID)).Only(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	if err := tx.Wallet.UpdateOne(account).
		SetAvailableFen(account.AvailableFen.Add(row.AmountFen)).
		Exec(ctx); err != nil {
		return decimal.Zero, err
	}

	if err := tx.LedgerEntry.Create().
		SetUserID(row.UserID).
		SetBusinessKey("creative:" + row.ID).
		SetKind(enum.LedgerSettlementCredit).
		SetReason(creativeKind(row)).
		SetAmountFen(row.AmountFen).
		SetReferenceID(row.ID).
		Exec(ctx); err != nil {
		return decimal.Zero, err
	}

	return row.AmountFen, tx.CreativeEarning.UpdateOne(row).SetBatchID(batch).Exec(ctx)
}

func (s *Service) monthCreative(ctx context.Context, start, end string, periods map[string]*WeekDTO, schedule task.MarketingSchedule, now time.Time, access MonthAccess, revisions map[string][]string, batches map[string]*ent.SettlementBatch, operators map[string]string) error {
	from, _ := time.ParseInLocation(time.DateOnly, start, now.Location())
	upto, _ := time.ParseInLocation(time.DateOnly, end, now.Location())
	ids := make([]string, 0, len(batches))
	for id := range batches {
		ids = append(ids, id)
	}

	query := s.Client.CreativeEarning.Query()
	paid := creativeearning.BatchIDIn(ids...)
	if access.Pending {
		query.Where(creativeearning.Or(paid, creativeearning.And(creativeearning.BatchIDEQ(""), creativeearning.EligibleAtGTE(from), creativeearning.EligibleAtLT(upto), creativeearning.AmountFenGT(decimal.Zero))))
	} else {
		query.Where(paid)
	}

	rows, err := query.Order(ent.Asc(creativeearning.FieldID)).All(ctx)
	if err != nil {
		return err
	}

	for _, row := range rows {
		w, err := s.Client.CreativeWork.Get(ctx, row.WorkID)
		if err != nil {
			return err
		}

		t, err := s.Client.CreativeTask.Get(ctx, w.TaskID)
		if err != nil {
			return err
		}

		date := app.BusinessDay(row.EligibleAt)
		line := WeekLineDTO{
			ID:           row.ID,
			AssignmentID: w.ID,
			UserID:       w.UserID,
			Title:        t.Title,
			Project:      creativeKind(row),
			TaskDate:     w.BusinessDate,
			PublishedOn:  app.BusinessDay(row.CreatedAt),
			Creative:     true,
			AmountFen:    row.AmountFen.String(),
			EligibleAt:   &row.EligibleAt,
			Due:          !now.Before(row.EligibleAt),
		}
		if batch := batches[row.BatchID]; batch != nil {
			date = batch.Through
			line.CreditedAt = &batch.CreatedAt
			line.CreditedBy = operators[batch.CreatorID]
		} else if line.Due {
			revisions[date] = append(revisions[date], creativeRevision(row))
		}

		if periods[date] == nil {
			periods[date] = newMonthPeriod(schedule, date, now, true)
		}

		periods[date].Due = periods[date].Due || line.Due
		periods[date].Lines = append(periods[date].Lines, line)
	}

	return nil
}
