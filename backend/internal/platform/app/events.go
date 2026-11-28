package app

import (
	"context"
	"strings"
	"sync/atomic"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/noticequeue"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type auditOriginKey struct{}

type auditOrigin struct {
	platform       enum.AuditPlatform
	endpoint       string
	eventAttempted atomic.Bool
}

func WithAuditOrigin(ctx context.Context, platform enum.AuditPlatform, endpoint string) context.Context {
	return context.WithValue(ctx, auditOriginKey{}, &auditOrigin{
		platform: platform,
		endpoint: endpoint,
	})
}

func AuditEventAttempted(ctx context.Context) bool {
	origin, ok := ctx.Value(auditOriginKey{}).(*auditOrigin)
	return ok && origin.eventAttempted.Load()
}

func AuditFlag(label string, value bool) string {
	if value {
		return label + "：是"
	}

	return label + "：否"
}

func Audit(ctx context.Context, tx *ent.Tx, actor string, action enum.AuditAction, ref, reason string, parameters ...string) error {
	_, err := tx.AuditEvent.Create().
		SetActorID(actor).
		SetAction(action).
		SetReferenceID(ref).
		SetReason(reason).
		SetParameters(strings.Join(parameters, "；")).
		Save(ctx)
	return err
}

func (s *Runtime) Notify(ctx context.Context, tx *ent.Tx, uid, title, body, ref string, cards ...wxnotice.Card) error {
	create := tx.Notification.Create().
		SetUserID(uid).
		SetTitle(title).
		SetBody(body).
		SetReferenceID(ref)
	if len(cards) > 0 {
		create.SetCard(cards[0]).SetDeliveryState(enum.DeliveryPending)
	}

	n, err := create.Save(ctx)
	if err != nil {
		return err
	}

	if len(cards) == 0 {
		return nil
	}

	ctx, span := otel.Tracer("yanxuan/queue").Start(ctx, "enqueue wechat_subscription", trace.WithSpanKind(trace.SpanKindProducer))
	defer func() { telemetry.Finish(span, err) }()
	telemetry.SetUser(ctx, n.UserID, enum.RoleUser)
	native, err := tx.SQLTx()
	if err != nil {
		return err
	}

	_, err = s.NoticeQueue.InsertTx(ctx, native, noticequeue.NewArgs(ctx, n.ID), nil)
	return err
}
