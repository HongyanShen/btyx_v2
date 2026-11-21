package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type CheckSubmission struct {
	Name              string
	Identity          string
	Phone             string
	Wechat            string
	ValidatedDocument Document
}

func RecordIdentityCheck(ctx context.Context, runtime *app.Runtime, tx *ent.Tx, userID string, action enum.AuditAction, reason string, submission CheckSubmission) error {
	var maskedName string
	if name := strings.TrimSpace(submission.Name); validIdentityName(name) {
		maskedName = MaskIdentityName(name)
	}

	var wechatCipher string
	if submission.Wechat != "" {
		var err error
		wechatCipher, err = runtime.Encrypt(submission.Wechat)
		if err != nil {
			return err
		}
	}

	return tx.AuditEvent.Create().
		SetActorID(userID).
		SetAction(action).
		SetReferenceID(userID).
		SetReason(reason).
		SetSubmittedPhone(MaskPhone(submission.Phone)).
		SetSubmittedWechatCipher(wechatCipher).
		SetSubmittedMaskedName(maskedName).
		SetSubmittedMaskedIdentity(MaskIdentityNumber(strings.ToUpper(strings.TrimSpace(submission.Identity)))).
		SetSubmittedFrontFileID(submission.ValidatedDocument.FrontFileID).
		SetSubmittedBackFileID(submission.ValidatedDocument.BackFileID).
		Exec(ctx)
}

func RecordFailedCheck(ctx context.Context, runtime *app.Runtime, userID string, action enum.AuditAction, failure error, submission CheckSubmission) error {
	reason := "实名核验暂时无法完成"
	var public *app.Error
	if errors.As(failure, &public) && public.Message != "" {
		reason = public.Message
	}

	recordContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()

	_, err := app.Transaction(recordContext, runtime, func(tx *ent.Tx) (bool, error) {
		return true, RecordIdentityCheck(recordContext, runtime, tx, userID, action, reason, submission)
	})
	if err != nil {
		return err
	}

	return failure
}
