package task

import (
	"context"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type MarketingPriceChange struct {
	BeforeFen string    `json:"before_fen"`
	AfterFen  string    `json:"after_fen"`
	ChangedAt time.Time `json:"changed_at"`
	Operator  string    `json:"operator"`
}

func readPriceChange(reason string) (string, string) {
	price, _, _ := strings.Cut(reason, "，")
	price = strings.TrimSuffix(strings.TrimPrefix(price, "每平台单价 "), " 分")
	before, after, found := strings.Cut(price, " → ")
	if !found {
		return "", price
	}

	return before, after
}

func marketingPriceChanges(ctx context.Context, client *ent.Client) ([]MarketingPriceChange, error) {
	const displayLimit = 5
	events, err := client.AuditEvent.Query().
		Where(
			auditevent.ActionEQ(enum.AuditMarketingSettings),
			auditevent.ReferenceIDEQ(marketingScope),
			auditevent.ReasonHasPrefix("每平台单价 "),
		).
		Select(auditevent.FieldID, auditevent.FieldActorID, auditevent.FieldCreatedAt, auditevent.FieldReason).
		Order(ent.Desc(auditevent.FieldCreatedAt), ent.Desc(auditevent.FieldID)).
		Limit(displayLimit + 1).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]MarketingPriceChange, 0, min(len(events), displayLimit))
	if len(events) == 0 {
		return result, nil
	}

	actorIDs := make([]string, 0, len(events))
	for _, event := range events {
		actorIDs = append(actorIDs, event.ActorID)
	}

	names, err := app.LookupOperatorNames(ctx, client, actorIDs)
	if err != nil {
		return nil, err
	}

	for index, event := range events[:min(len(events), displayLimit)] {
		before, after := readPriceChange(event.Reason)
		if before == "" {
			before = "750"
			if index+1 < len(events) {
				_, before = readPriceChange(events[index+1].Reason)
			}
		}

		result = append(result, MarketingPriceChange{
			BeforeFen: before,
			AfterFen:  after,
			ChangedAt: event.CreatedAt,
			Operator:  names.Label(event.ActorID),
		})
	}

	return result, nil
}

type SettingsChange struct {
	ChangedAt time.Time
	Operator  string
	Change    string
}

func (s *Service) MarketingHistory(ctx context.Context, page int) ([]SettingsChange, bool, error) {
	const size = 10
	rows, err := s.Client.AuditEvent.Query().Where(auditevent.ActionEQ(enum.AuditMarketingSettings), auditevent.ReferenceIDEQ(marketingScope)).Order(ent.Desc(auditevent.FieldCreatedAt), ent.Desc(auditevent.FieldID)).Offset((page - 1) * size).Limit(size + 1).All(ctx)
	if err != nil {
		return nil, false, err
	}

	more := len(rows) > size
	rows = rows[:min(len(rows), size)]
	var ids []string
	for _, row := range rows {
		ids = append(ids, row.ActorID)
	}

	names, err := app.LookupOperatorNames(ctx, s.Client, ids)
	if err != nil {
		return nil, false, err
	}

	var result []SettingsChange
	for _, row := range rows {
		result = append(result, SettingsChange{
			ChangedAt: app.BeijingTime(row.CreatedAt),
			Operator:  names.Label(row.ActorID),
			Change:    row.Reason,
		})
	}

	return result, more, nil
}
