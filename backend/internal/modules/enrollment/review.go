package enrollment

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/marketingtags"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
)

func (s *Service) ReviewEnrollment(ctx context.Context, actorID, userID string, ok bool, reason string, tags []string, updatedAt *time.Time) error {
	return s.SaveEnrollmentReview(ctx, actorID, userID, ReviewInput{
		Approved:  ok,
		Reason:    reason,
		Tags:      tags,
		UpdatedAt: updatedAt,
	})
}

func (s *Service) SaveEnrollmentReview(ctx context.Context, actorID, userID string, input ReviewInput) error {
	if input.TagsOnly {
		return s.updateEnrollmentTags(ctx, actorID, userID, input)
	}

	input.Reason = strings.TrimSpace(input.Reason)
	if !input.Approved && !input.PortraitOnly && input.Reason == "" {
		return app.Invalid("驳回必须填写原因")
	}

	if input.AgeGroup != "" && !input.AgeGroup.Valid() {
		return app.Invalid("年龄层无效")
	}

	if input.PortraitOnly && (input.UpdatedAt == nil || input.RecentPost == nil || input.NewAccount == nil || input.AgeGroup == "") {
		return app.Invalid("请刷新报名并填写完整的人工复核")
	}

	status := enum.EnrollmentRejected
	if input.Approved || input.PortraitOnly {
		status = enum.EnrollmentApproved
	}

	expectedRevision := input.UpdatedAt
	var document identity.Document
	var submission identity.CheckSubmission
	var checkedLegacyApplication bool
	if input.Approved && !input.PortraitOnly {
		member, err := s.Client.User.Query().
			Where(user.IDEQ(userID)).
			WithEnrollment(enrollmentstate.LoadCurrent).
			WithVerification().
			Only(ctx)
		if err != nil {
			return app.WrapMissing(err)
		}

		record, err := reviewTarget(ctx, s.Client, member, input.EnrollmentID)
		if err != nil {
			return err
		}

		application, err := reviewableEnrollment(record, status, input, expectedRevision)
		if err != nil || application == nil {
			return err
		}

		expectedRevision = &application.UpdatedAt
		input.EnrollmentID = application.ID
		if !identity.IsVerified(member) {
			document = enrollmentDocument(member, application)
			if application.IdentityCheckedAt == nil {
				submission = identity.CheckSubmission{
					Name:              s.Decrypt(document.NameCipher),
					Identity:          s.Decrypt(document.IdentityCipher),
					ValidatedDocument: document,
				}
				if err := document.Check(ctx, s.Runtime, s.Verifier); err != nil {
					return identity.RecordFailedCheck(ctx, s.Runtime, userID, enum.AuditEnrollmentIdentityFailed, err, submission)
				}

				checkedLegacyApplication = true
			}
		}
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		currentUser, err := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			WithEnrollment(enrollmentstate.LoadCurrent).
			WithVerification().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		record, err := reviewTarget(ctx, tx.Client(), currentUser, input.EnrollmentID)
		if err != nil {
			return false, err
		}

		enrollment, err := reviewableEnrollment(record, status, input, expectedRevision)
		if err != nil || enrollment == nil {
			return false, err
		}

		if input.Approved && !input.PortraitOnly && !identity.IsVerified(currentUser) {
			if document.IdentityCipher == "" {
				return false, app.Conflict("实名状态已更新，请刷新后重新审核")
			}

			if err := identity.SaveDocument(ctx, tx, userID, document, s.Now()); err != nil {
				return false, err
			}

			if checkedLegacyApplication {
				if err := identity.RecordIdentityCheck(ctx, s.Runtime, tx, userID, enum.AuditEnrollmentIdentityVerify, "", submission); err != nil {
					return false, err
				}
			}
		}

		update := tx.UserEnrollment.UpdateOne(enrollment).
			SetNillableRecentPost(input.RecentPost).
			SetNillableNewAccount(input.NewAccount)
		if input.AgeGroup != "" {
			update.SetAgeGroup(input.AgeGroup)
		}

		if !input.PortraitOnly {
			update.SetStatus(status).SetReason(input.Reason)
		}

		if input.Approved || input.PortraitOnly || input.Tags != nil {
			if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
				return false, err
			}

			update.SetTags(input.Tags)
		}

		if err := update.Exec(ctx); err != nil {
			return false, err
		}

		if input.PortraitOnly {
			return true, app.Audit(ctx, tx, actorID, enum.AuditEnrollmentPortrait, userID, "", fmt.Sprintf("标签：%v", input.Tags))
		}

		message := input.Reason
		if input.Approved {
			message = "报名已通过，请前往小程序查看可领取的任务"
		}

		card := wxnotice.EnrollmentCard(input.Approved, input.Reason)
		if err = s.Notify(ctx, tx, userID, "报名审核结果", message, enrollment.ID, card); err != nil {
			return false, err
		}

		auditAction := enum.AuditEnrollmentRejected
		if input.Approved {
			auditAction = enum.AuditEnrollmentApproved
		}

		return true, app.Audit(ctx, tx, actorID, auditAction, userID, input.Reason, fmt.Sprintf("标签：%v", input.Tags))
	})
	return err
}

func (s *Service) updateEnrollmentTags(ctx context.Context, actorID, userID string, input ReviewInput) error {
	if input.Tags == nil || input.UpdatedAt == nil || input.Approved || input.PortraitOnly {
		return app.Invalid("请提交要保存的标签和报名版本")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		member, err := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			WithEnrollment(enrollmentstate.LoadCurrent).
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		application, err := reviewTarget(ctx, tx.Client(), member, input.EnrollmentID)
		if err != nil {
			return false, err
		}

		if application == nil || application.Status == enum.EnrollmentNone {
			return false, app.Conflict("找不到可编辑标签的报名")
		}

		if slices.Equal(application.Tags, input.Tags) {
			return false, nil
		}

		if !input.UpdatedAt.Equal(application.UpdatedAt) {
			return false, app.Conflict("报名资料已更新，请刷新后重新编辑")
		}

		if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
			return false, err
		}

		if err := tx.UserEnrollment.UpdateOne(application).SetTags(input.Tags).Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditEnrollmentPortrait, userID, "调整标签", fmt.Sprintf("标签：%v", input.Tags))
	})
	return err
}

func reviewTarget(ctx context.Context, client *ent.Client, member *ent.User, enrollmentID string) (*ent.UserEnrollment, error) {
	current := enrollmentstate.Current(member)
	if enrollmentID == "" || current != nil && current.ID == enrollmentID {
		return current, nil
	}

	record, err := client.UserEnrollment.Query().
		Where(userenrollment.IDEQ(enrollmentID), userenrollment.UserIDEQ(member.ID)).
		Only(ctx)
	return record, app.WrapMissing(err)
}

func reviewableEnrollment(application *ent.UserEnrollment, status enum.EnrollmentStatus, input ReviewInput, expectedRevision *time.Time) (*ent.UserEnrollment, error) {
	if application == nil {
		return nil, app.Conflict("找不到可复核的报名")
	}

	if application.Status == status && sameReview(application, input) {
		return nil, nil
	}

	if expectedRevision != nil && !expectedRevision.Equal(application.UpdatedAt) {
		return nil, app.Conflict("报名资料已更新，请刷新后重新审核")
	}

	if input.PortraitOnly {
		if application.Status != enum.EnrollmentApproved {
			return nil, app.Conflict("仅已通过报名可修改画像")
		}

		return application, nil
	}

	if application.IsCurrent && application.Status == enum.EnrollmentPending {
		return application, nil
	}

	return nil, app.Conflict("此报名状态已更新，请刷新后处理")
}

func sameReview(application *ent.UserEnrollment, input ReviewInput) bool {
	if !input.PortraitOnly && application.Reason != input.Reason {
		return false
	}

	if (input.Approved || input.PortraitOnly || input.Tags != nil) && !slices.Equal(application.Tags, input.Tags) {
		return false
	}

	return (input.RecentPost == nil || application.RecentPost != nil && *application.RecentPost == *input.RecentPost) &&
		(input.NewAccount == nil || application.NewAccount != nil && *application.NewAccount == *input.NewAccount) &&
		(input.AgeGroup == "" || application.AgeGroup != nil && *application.AgeGroup == input.AgeGroup)
}

func enrollmentDocument(member *ent.User, application *ent.UserEnrollment) identity.Document {
	if application.NameCipher == "" && application.IdentityCipher == "" {
		// Older applications kept their unverified identity details on the verification record.
		if verification := member.Edges.Verification; verification != nil {
			return identity.Document{
				NameCipher:     verification.NameCipher,
				IdentityCipher: verification.IdentityCipher,
				FrontFileID:    verification.FrontFileID,
				BackFileID:     verification.BackFileID,
			}
		}
	}

	return identity.Document{
		NameCipher:     application.NameCipher,
		IdentityCipher: application.IdentityCipher,
		FrontFileID:    application.FrontFileID,
		BackFileID:     application.BackFileID,
	}
}
