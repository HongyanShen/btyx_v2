package identity

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type VerifyIdentityInput struct {
	DocumentInput
	Wechat              *string `json:"wechat"`
	ContactPhone        *string `json:"contact_phone"`
	ContactPhoneReceipt string  `json:"contact_phone_receipt"`
	CorrectIdentity     bool    `json:"correct_identity"`
}

func (s *Service) VerifyIdentity(ctx context.Context, userID string, input VerifyIdentityInput) error {
	var submission CheckSubmission
	if input.Wechat != nil {
		wechat := strings.TrimSpace(*input.Wechat)
		if utf8.RuneCountInString(wechat) > 50 || strings.ContainsFunc(wechat, unicode.IsControl) {
			return app.Invalid("请输入有效的微信号")
		}

		input.Wechat = &wechat
		submission.Wechat = wechat
	}

	if input.ContactPhone != nil {
		contactPhone := strings.TrimSpace(*input.ContactPhone)
		if contactPhone != "" && !verifiedPhonePattern.MatchString(contactPhone) {
			return app.Invalid("请输入有效的联系手机号")
		}

		input.ContactPhone = &contactPhone
		submission.Phone = contactPhone
	}

	if input.ContactPhoneReceipt != "" {
		if input.ContactPhone != nil {
			return app.Invalid("请只提交一种联系手机号")
		}

		contactPhone, err := s.contactPhoneFromReceipt(userID, input.ContactPhoneReceipt)
		if err != nil {
			return err
		}

		input.ContactPhone = &contactPhone
		submission.Phone = contactPhone
	}

	member, err := s.Client.User.Query().
		Where(user.IDEQ(userID)).
		WithVerification().
		WithEnrollment(enrollmentstate.LoadCurrent).
		Only(ctx)
	if err != nil {
		return app.WrapMissing(err)
	}

	if member.Blocked {
		return app.Forbidden("账号已暂停")
	}

	previous := member.Edges.Verification
	if input.CorrectIdentity && strings.TrimSpace(input.Identity) == "" &&
		previous != nil && previous.VerifiedAt != nil &&
		input.FrontFileID != "" && input.FrontFileID == previous.FrontFileID {
		input.Identity = s.Decrypt(previous.IdentityCipher)
	}

	var document Document
	if s.needsDocumentCheck(member, input) {
		submission.Name = input.Name
		submission.Identity = input.Identity
		document, err = PrepareDocument(ctx, s.Client, s.Runtime, userID, input.DocumentInput)
		if err != nil {
			return RecordFailedCheck(ctx, s.Runtime, userID, enum.AuditUserIdentityFailed, err, submission)
		}

		submission.ValidatedDocument = document
		if err := document.Check(ctx, s.Runtime, s.Verifier); err != nil {
			return RecordFailedCheck(ctx, s.Runtime, userID, enum.AuditUserIdentityFailed, err, submission)
		}
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		member, err := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			WithVerification().
			WithEnrollment(enrollmentstate.LoadCurrent).
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if member.Blocked {
			return false, app.Forbidden("账号已暂停")
		}

		verifying := s.needsDocumentCheck(member, input)
		if verifying {
			if document.IdentityCipher == "" {
				return false, app.Conflict("实名状态已更新，请刷新后重试")
			}

			if input.CorrectIdentity {
				current := member.Edges.Verification
				if identityDocumentChanged(previous, current) {
					return false, app.Conflict("实名资料已更新，请重新确认后提交")
				}

				if current != nil {
					if err := tx.FileObject.Update().
						Where(
							fileobject.OwnerIDEQ(userID),
							fileobject.IDIn(current.FrontFileID, current.BackFileID),
							fileobject.IDNotIn(document.FrontFileID, document.BackFileID),
						).
						SetIdentityPrivate(true).
						Exec(ctx); err != nil {
						return false, err
					}
				}
			}

			if err := SaveDocument(ctx, tx, userID, document, s.Now()); err != nil {
				return false, err
			}
		}

		contactsChanged, err := s.saveIdentityContacts(ctx, tx, userID, member.Edges.Verification, input)
		if err != nil {
			return false, err
		}

		if verifying {
			return true, RecordIdentityCheck(ctx, s.Runtime, tx, userID, enum.AuditUserIdentityVerify, "", submission)
		}

		if contactsChanged {
			return true, app.Audit(ctx, tx, userID, enum.AuditUserProfileUpdate, userID, "")
		}

		return false, nil
	})
	return err
}

func (s *Service) saveIdentityContacts(ctx context.Context, tx *ent.Tx, userID string, verification *ent.UserVerification, input VerifyIdentityInput) (bool, error) {
	var currentWechat, currentContactPhone string
	if verification != nil {
		currentWechat = s.Decrypt(verification.WechatCipher)
		currentContactPhone = s.Decrypt(verification.ContactPhoneCipher)
	}

	wechatChanged := input.Wechat != nil && *input.Wechat != currentWechat
	contactChanged := input.ContactPhone != nil && *input.ContactPhone != currentContactPhone
	if !wechatChanged && !contactChanged {
		return false, nil
	}

	contact := tx.UserVerification.Create().SetUserID(userID)
	if wechatChanged {
		cipher, err := s.Encrypt(*input.Wechat)
		if err != nil {
			return false, err
		}

		contact.SetWechatCipher(cipher)
	}

	if contactChanged {
		var cipher string
		if *input.ContactPhone != "" {
			var err error
			cipher, err = s.Encrypt(*input.ContactPhone)
			if err != nil {
				return false, err
			}
		}

		contact.SetContactPhoneCipher(cipher)
	}

	save := contact.OnConflictColumns(userverification.FieldUserID).SetUpdatedAt(s.Now())
	if wechatChanged {
		save.UpdateWechatCipher()
	}

	if contactChanged {
		save.UpdateContactPhoneCipher()
	}

	return true, save.Exec(ctx)
}

func (s *Service) needsDocumentCheck(member *ent.User, input VerifyIdentityInput) bool {
	if !IsVerified(member) {
		return true
	}

	if !input.CorrectIdentity {
		return false
	}

	record := member.Edges.Verification
	return record == nil || record.VerifiedAt == nil ||
		record.FrontFileID != input.FrontFileID || record.BackFileID != input.BackFileID ||
		s.Decrypt(record.NameCipher) != strings.TrimSpace(input.Name) ||
		s.Decrypt(record.IdentityCipher) != strings.ToUpper(strings.TrimSpace(input.Identity))
}

func identityDocumentChanged(previous, current *ent.UserVerification) bool {
	if previous == nil || current == nil {
		return previous != current
	}

	return previous.IdentityCipher != current.IdentityCipher
}
