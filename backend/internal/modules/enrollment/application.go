package enrollment

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userplatformprofile"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/marketingtags"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

var contactPhonePattern = regexp.MustCompile(`^1[0-9]{10}$`)

var xiaohongshuNumberPattern = regexp.MustCompile(`^[^\p{Z}\p{Cc}:/\\<>?#：／]+$`)

func (s *Service) Apply(ctx context.Context, userID string, input ApplyInput) error {
	if input.AgreementID == "" {
		return app.Invalid("请先阅读并同意营销分发任务协议")
	}

	input.Douyin = strings.TrimSpace(input.Douyin)
	input.XHS = strings.TrimSpace(input.XHS)
	if input.Douyin == "" || utf8.RuneCountInString(input.Douyin) > 100 || input.XHS == "" || len(input.XHS) > 512 {
		return app.Invalid("请填写抖音号和小红书号")
	}

	input.Phone = strings.TrimSpace(input.Phone)
	if input.Phone != "" && !contactPhonePattern.MatchString(input.Phone) {
		return app.Invalid("请输入有效的手机号")
	}

	member, err := s.Client.User.Query().
		Where(user.IDEQ(userID)).
		WithEnrollment(enrollmentstate.LoadCurrent).
		WithVerification().
		WithPlatformProfile().
		Only(ctx)
	if err != nil {
		return app.WrapMissing(err)
	}

	previous := enrollmentstate.Current(member)
	if previous != nil && previous.IdentityCheckedAt != nil &&
		input.Name == "" && input.Identity == "" &&
		input.FrontFileID != "" && input.FrontFileID == previous.FrontFileID {
		input.Name = s.Decrypt(previous.NameCipher)
		input.Identity = s.Decrypt(previous.IdentityCipher)
	}

	if proceed, err := s.canApply(ctx, s.Client, member, input); !proceed {
		return err
	}

	if !xiaohongshuNumberPattern.MatchString(input.XHS) {
		return app.Invalid("请填写小红书号，可在小红书「我」页面查看")
	}

	if err := agreement.CheckCurrentVersion(ctx, s.Client.Agreement.Query(), input.AgreementID); err != nil {
		return err
	}

	submission := identity.CheckSubmission{
		Name:     input.Name,
		Identity: input.Identity,
		Phone:    input.Phone,
	}
	var document identity.Document
	var checkedAt *time.Time
	if !identity.IsVerified(member) {
		document, err = identity.PrepareDocument(ctx, s.Client, s.Runtime, userID, input.DocumentInput)
		if err != nil {
			return identity.RecordFailedCheck(ctx, s.Runtime, userID, enum.AuditEnrollmentIdentityFailed, err, submission)
		}

		submission.ValidatedDocument = document
		if err := document.Check(ctx, s.Runtime, s.Verifier); err != nil {
			return identity.RecordFailedCheck(ctx, s.Runtime, userID, enum.AuditEnrollmentIdentityFailed, err, submission)
		}

		checkTime := s.Now()
		checkedAt = &checkTime
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		current, err := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			WithEnrollment(enrollmentstate.LoadCurrent).
			WithVerification().
			WithPlatformProfile().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if proceed, err := s.canApply(ctx, tx.Client(), current, input); !proceed {
			return false, err
		}

		enrollment := enrollmentstate.Current(current)
		if (previous == nil) != (enrollment == nil) ||
			previous != nil && (previous.ID != enrollment.ID || !previous.UpdatedAt.Equal(enrollment.UpdatedAt)) {
			return false, app.Conflict("报名状态已更新，请刷新后重新提交")
		}

		if checkedAt == nil && !identity.IsVerified(current) {
			return false, app.Conflict("实名状态已更新，请刷新后重新提交")
		}

		if err := agreement.ConfirmCurrentVersion(ctx, tx, userID, input.AgreementID); err != nil {
			return false, err
		}

		var contactPhoneCipher string
		if input.Phone != "" {
			contactPhoneCipher, err = s.Encrypt(input.Phone)
			if err != nil {
				return false, err
			}
		}

		if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
			return false, err
		}

		now := s.Now()
		if enrollment != nil {
			if err := archiveApplication(ctx, tx, current, enrollment); err != nil {
				return false, err
			}
		}

		documentCheckedAt := checkedAt
		if verification := current.Edges.Verification; documentCheckedAt == nil && verification != nil {
			document = identity.Document{
				NameCipher:     verification.NameCipher,
				IdentityCipher: verification.IdentityCipher,
				FrontFileID:    verification.FrontFileID,
				BackFileID:     verification.BackFileID,
			}
			documentCheckedAt = verification.VerifiedAt
		}

		err = tx.UserPlatformProfile.Create().
			SetUserID(userID).
			SetDouyin(input.Douyin).
			SetXhs(input.XHS).
			OnConflictColumns(userplatformprofile.FieldUserID).
			UpdateDouyin().
			UpdateXhs().
			SetUpdatedAt(now).
			Exec(ctx)
		if err != nil {
			return false, err
		}

		err = tx.UserEnrollment.Create().
			SetUserID(userID).
			SetApplicantNickname(current.Nickname).
			SetDouyin(input.Douyin).
			SetXhs(input.XHS).
			SetContactPhoneCipher(contactPhoneCipher).
			SetNameCipher(document.NameCipher).
			SetIdentityCipher(document.IdentityCipher).
			SetFrontFileID(document.FrontFileID).
			SetBackFileID(document.BackFileID).
			SetNillableIdentityCheckedAt(documentCheckedAt).
			SetSubmittedAt(now).
			SetTags(input.Tags).
			SetStatus(enum.EnrollmentPending).
			SetUpdatedAt(now).
			Exec(ctx)
		if err != nil {
			return false, err
		}

		if checkedAt != nil {
			if err := identity.RecordIdentityCheck(ctx, s.Runtime, tx, userID, enum.AuditEnrollmentIdentityVerify, "", submission); err != nil {
				return false, err
			}
		}

		return true, app.Audit(ctx, tx, userID, enum.AuditEnrollmentApply, userID, "")
	})
	return err
}

func archiveApplication(ctx context.Context, tx *ent.Tx, member *ent.User, application *ent.UserEnrollment) error {
	update := tx.UserEnrollment.UpdateOne(application).
		SetIsCurrent(false).
		SetUpdatedAt(application.UpdatedAt)
	if platforms := member.Edges.PlatformProfile; application.Douyin == nil && platforms != nil {
		update.SetDouyin(platforms.Douyin).SetXhs(platforms.Xhs)
	}

	if application.SubmittedAt == nil && application.Status != enum.EnrollmentNone {
		submittedAt := application.CreatedAt
		event, err := tx.AuditEvent.Query().
			Where(auditevent.ReferenceIDEQ(member.ID), auditevent.ActionEQ(enum.AuditEnrollmentApply)).
			Order(ent.Desc(auditevent.FieldCreatedAt)).
			First(ctx)
		if err == nil {
			submittedAt = event.CreatedAt
		} else if !ent.IsNotFound(err) {
			return err
		}

		update.SetSubmittedAt(submittedAt)
	}

	return update.Exec(ctx)
}

func (s *Service) canApply(ctx context.Context, client *ent.Client, member *ent.User, input ApplyInput) (bool, error) {
	if member.Blocked {
		return false, app.Forbidden("账号已暂停")
	}

	enrollment := enrollmentstate.Current(member)
	if enrollment == nil || enrollment.Status == enum.EnrollmentNone {
		if input.PreviousEnrollmentID == "" {
			return true, nil
		}

		return false, app.Conflict("报名状态已更新，请刷新后重新提交")
	}

	switch enrollment.Status {
	case enum.EnrollmentApproved:
		return false, app.Conflict("已通过审核，资料变更请通过反馈申请复核")
	case enum.EnrollmentPending:
		if s.sameApplication(member, input) {
			signed, err := agreement.HasConsent(ctx, client, member.ID, input.AgreementID)
			if err != nil || signed {
				return false, err
			}
		}

		return false, app.Conflict("报名审核中，请等待审核结果")
	case enum.EnrollmentRejected:
		if input.PreviousEnrollmentID == enrollment.ID {
			return true, nil
		}

		return false, app.Conflict("请刷新驳回结果，再重新提交报名")
	default:
		return false, app.Conflict("报名状态已更新，请刷新后重新提交")
	}
}

func (s *Service) sameApplication(member *ent.User, input ApplyInput) bool {
	enrollment := enrollmentstate.Current(member)
	platforms := member.Edges.PlatformProfile
	if platforms == nil || platforms.Douyin != input.Douyin || platforms.Xhs != input.XHS ||
		(input.Tags != nil && !slices.Equal(input.Tags, enrollment.Tags)) ||
		input.Phone != s.Decrypt(enrollment.ContactPhoneCipher) {
		return false
	}

	if identity.IsVerified(member) {
		return true
	}

	return enrollment.FrontFileID == input.FrontFileID && enrollment.BackFileID == input.BackFileID &&
		s.Decrypt(enrollment.NameCipher) == strings.TrimSpace(input.Name) &&
		s.Decrypt(enrollment.IdentityCipher) == strings.ToUpper(strings.TrimSpace(input.Identity))
}
