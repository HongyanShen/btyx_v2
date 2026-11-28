package notification

import (
	"context"
	"errors"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/ent/subscriptionchoice"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Sender interface {
	SendCard(context.Context, string, wxnotice.Card) (int, error)
}

func (s *Service) Deliver(ctx context.Context, id, openid string, sender Sender, lastAttempt bool) error {
	n, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (*ent.Notification, error) {
		record, err := tx.Notification.Query().Where(notification.IDEQ(id)).ForUpdate().Only(ctx)
		if err != nil {
			return nil, err
		}

		if record.DeliveryState == enum.DeliverySending {
			return nil, tx.Notification.UpdateOne(record).
				SetDeliveryState(enum.DeliveryUnknown).
				SetDeliveryReason("上次发送中断，结果待核实").
				Exec(ctx)
		}

		if record.DeliveryState != enum.DeliveryPending {
			return nil, nil
		}

		if field := record.Card.FieldError(); field != "" {
			return nil, tx.Notification.UpdateOne(record).
				SetDeliveryState(enum.DeliveryFailed).
				SetDeliveryReason("微信模板字段不符合长度或格式要求：" + field).
				Exec(ctx)
		}

		if _, err = tx.User.Query().Where(user.IDEQ(record.UserID)).ForUpdate().Only(ctx); err != nil {
			return nil, err
		}

		choices := tx.SubscriptionChoice.Query().
			Where(subscriptionchoice.UserIDEQ(record.UserID), subscriptionchoice.TemplateIDEQ(record.Card.TemplateID))
		// A refusal or a disabled switch takes precedence over earlier grants.
		refusal, err := choices.Clone().
			Where(subscriptionchoice.Or(subscriptionchoice.ResultIn(enum.SubscriptionReject, enum.SubscriptionBan), subscriptionchoice.MainSwitchEQ(false), subscriptionchoice.RememberedIn("reject", "ban"))).
			Order(ent.Desc(subscriptionchoice.FieldObservedAt)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, err
		}

		grants := choices.Clone().Where(
			subscriptionchoice.ResultEQ(enum.SubscriptionAccept),
			subscriptionchoice.Or(
				subscriptionchoice.CreatedAtLTE(record.CreatedAt),
				subscriptionchoice.ObservedAtLTE(record.CreatedAt),
			),
		)
		if refusal != nil {
			grants.Where(subscriptionchoice.ObservedAtGT(refusal.ObservedAt))
		}

		if record.ChoiceID != "" {
			grants.Where(subscriptionchoice.IDEQ(record.ChoiceID))
		} else {
			grants.Where(subscriptionchoice.UsedByEQ(""))
		}

		grant, err := grants.Order(ent.Asc(subscriptionchoice.FieldObservedAt)).First(ctx)
		if ent.IsNotFound(err) {
			reason := "未记录同意订阅"
			accepted, readErr := choices.Clone().Where(subscriptionchoice.ResultEQ(enum.SubscriptionAccept)).Exist(ctx)
			if readErr != nil {
				return nil, readErr
			}

			if accepted {
				reason = "该事件没有可用的一次性授权"
			}

			if refusal != nil {
				renewed, readErr := choices.Clone().
					Where(subscriptionchoice.ResultEQ(enum.SubscriptionAccept), subscriptionchoice.ObservedAtGT(refusal.ObservedAt), subscriptionchoice.ObservedAtLTE(record.CreatedAt)).
					Exist(ctx)
				if readErr != nil {
					return nil, readErr
				}

				if !renewed {
					switch {
					case refusal.MainSwitch != nil && !*refusal.MainSwitch:
						reason = "微信通知总开关已关闭"
					case refusal.Result == enum.SubscriptionBan || refusal.Remembered == "ban":
						reason = "微信已停用此模板"
					default:
						reason = "已拒绝此项订阅，可在微信设置中修改"
					}
				}
			}

			return nil, tx.Notification.UpdateOne(record).
				SetDeliveryState(enum.DeliverySkipped).
				SetDeliveryReason(reason).
				Exec(ctx)
		}

		if err != nil {
			return nil, err
		}

		if err = tx.SubscriptionChoice.UpdateOne(grant).SetUsedBy(id).Exec(ctx); err != nil {
			return nil, err
		}

		return tx.Notification.UpdateOne(record).
			SetChoiceID(grant.ID).
			SetDeliveryState(enum.DeliverySending).
			Save(ctx)
	})
	if err != nil || n == nil {
		return err
	}

	code, sendErr := sender.SendCard(ctx, openid, n.Card)
	state := enum.DeliverySent
	reason := ""
	var retry error
	switch {
	case errors.Is(sendErr, wxnotice.ErrBeforeSend) || code == -1 || code == 40001 || code == 42001 || code == 45009:
		state = enum.DeliveryPending
		reason = "微信暂未受理，等待重试"
		retry = errors.New("微信暂未受理订阅消息")
		if lastAttempt {
			state = enum.DeliveryFailed
			reason = "重试次数已用完"
		}
	case sendErr != nil:
		state = enum.DeliveryUnknown
		reason = "未收到明确发送结果"
	case code == 43101:
		state = enum.DeliverySkipped
		reason = "微信返回未订阅或授权已用完"
	case code != 0:
		state = enum.DeliveryFailed
		reason = "微信拒绝此条消息"
	}

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int("wechat.errcode", code), attribute.String("notification.outcome", string(state)))
	if sendErr != nil || code != 0 {
		span.SetStatus(codes.Error, "")
	}

	if err = s.Client.Notification.UpdateOneID(id).
		SetDeliveryState(state).
		SetDeliveryReason(reason).
		SetWechatCode(code).
		Exec(ctx); err != nil {
		return err
	}

	return retry
}
