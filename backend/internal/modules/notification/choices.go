package notification

import (
	"context"
	"slices"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/subscriptionchoice"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
)

type Choice struct {
	TemplateID string                  `json:"template_id"`
	Result     enum.SubscriptionResult `json:"result"`
	Remembered string                  `json:"remembered"`
}

type ChoiceBatch struct {
	Attempt    string    `json:"attempt"`
	Scene      string    `json:"scene"`
	ObservedAt time.Time `json:"observed_at"`
	MainSwitch *bool     `json:"main_switch"`
	ErrorCode  int       `json:"error_code"`
	Choices    []Choice  `json:"choices"`
}

func (s *Service) RecordChoices(ctx context.Context, uid string, in ChoiceBatch) error {
	allowed := wxnotice.Scenes[in.Scene]
	if len(in.Choices) == 0 || len(in.Choices) > 3 || len(in.Attempt) > 80 || len(in.Attempt) < 8 || in.ObservedAt.IsZero() || in.ObservedAt.After(s.Now().Add(5*time.Minute)) {
		return app.Invalid("订阅操作无效")
	}

	seen := map[string]bool{}
	for _, c := range in.Choices {
		if seen[c.TemplateID] || !slices.Contains(allowed, c.TemplateID) || !c.Result.Valid() || !slices.Contains([]string{"", "accept", "reject", "ban"}, c.Remembered) {
			return app.Invalid("订阅选项无效")
		}

		seen[c.TemplateID] = true
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		if _, err := tx.User.Query().Where(user.IDEQ(uid)).ForUpdate().Only(ctx); err != nil {
			return false, err
		}

		for _, c := range in.Choices {
			old, err := tx.SubscriptionChoice.Query().
				Where(subscriptionchoice.UserIDEQ(uid), subscriptionchoice.AttemptEQ(in.Attempt), subscriptionchoice.TemplateIDEQ(c.TemplateID)).
				Only(ctx)
			if err == nil {
				if old.Result != c.Result || old.Scene != in.Scene {
					return false, app.Conflict("该订阅操作已有其他结果")
				}

				continue
			}

			if !ent.IsNotFound(err) {
				return false, err
			}

			err = tx.SubscriptionChoice.Create().
				SetUserID(uid).
				SetScene(in.Scene).
				SetAttempt(in.Attempt).
				SetTemplateID(c.TemplateID).
				SetResult(c.Result).
				SetRemembered(c.Remembered).
				SetObservedAt(in.ObservedAt).
				SetNillableMainSwitch(in.MainSwitch).
				SetErrorCode(in.ErrorCode).
				Exec(ctx)
			if err != nil {
				return false, err
			}
		}

		return true, nil
	})
	return err
}
