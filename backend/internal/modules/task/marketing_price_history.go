package task

import (
	"context"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
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

	operators, err := client.Operator.Query().
		Where(operator.IDIn(actorIDs...)).
		Select(operator.FieldID, operator.FieldUsername).
		All(ctx)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(operators))
	for _, account := range operators {
		names[account.ID] = account.Username
	}

	for index, event := range events[:min(len(events), displayLimit)] {
		before, after := readPriceChange(event.Reason)
		if before == "" {
			before = "750"
			if index+1 < len(events) {
				_, before = readPriceChange(events[index+1].Reason)
			}
		}

		name := names[event.ActorID]
		if name == "" {
			name = "运营账号"
		}

		result = append(result, MarketingPriceChange{
			BeforeFen: before,
			AfterFen:  after,
			ChangedAt: event.CreatedAt,
			Operator:  name,
		})
	}

	return result, nil
}
