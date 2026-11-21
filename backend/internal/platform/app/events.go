package app

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/logging"

	"baituber.cn/yanxuan/v2/ent"
)

func Audit(ctx context.Context, tx *ent.Tx, actor string, action enum.AuditAction, ref, reason string) error {
	_, err := tx.AuditEvent.Create().
		SetActorID(actor).
		SetAction(action).
		SetReferenceID(ref).
		SetReason(reason).
		Save(ctx)
	return err
}

func Notify(ctx context.Context, tx *ent.Tx, uid, title, body, ref string) error {
	n, err := tx.Notification.Create().
		SetUserID(uid).
		SetTitle(title).
		SetBody(body).
		SetReferenceID(ref).
		Save(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Job.Create().
		SetKind(enum.JobNotification).
		SetReferenceID(n.ID).
		SetRequestID(logging.RequestID(ctx)).
		Save(ctx)
	return err
}
