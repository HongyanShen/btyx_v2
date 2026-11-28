package identity

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

var verifiedPhonePattern = regexp.MustCompile(`^\+?[0-9]{8,15}$`)

func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) error {
	if input.Nickname == nil && input.AvatarFileID == nil {
		return app.Invalid("请填写要保存的个人资料")
	}

	var nickname string
	if input.Nickname != nil {
		nickname = strings.TrimSpace(*input.Nickname)
		if nickname == "" || utf8.RuneCountInString(nickname) > 32 || strings.ContainsFunc(nickname, unicode.IsControl) {
			return app.Invalid("请输入 1 至 32 个字符的昵称")
		}
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		member, err := lockedProfile(ctx, tx, userID)
		if err != nil {
			return false, err
		}

		update := tx.User.UpdateOne(member)
		changed := input.Nickname != nil && nickname != member.Nickname
		if changed {
			update.SetNickname(nickname)
		}

		if input.AvatarFileID != nil && *input.AvatarFileID != member.AvatarFileID {
			allowed, err := tx.FileObject.Query().
				Where(
					fileobject.IDEQ(*input.AvatarFileID),
					fileobject.OwnerIDEQ(userID),
					fileobject.StatusEQ(enum.FileReady),
					fileobject.StorageEQ(enum.FileStorageTOS),
					fileobject.VerifiedAtNotNil(),
					fileobject.ObjectKeyNEQ(""),
					fileobject.MediaTypeIn(enum.FileJPEG, enum.FilePNG, enum.FileGIF, enum.FileWEBP),
				).
				Exist(ctx)
			if err != nil {
				return false, err
			}

			if !allowed {
				return false, app.Invalid("头像必须是本人已确认上传的图片")
			}

			update.SetAvatarFileID(*input.AvatarFileID)
			changed = true
		}

		if !changed {
			return false, nil
		}

		if err := update.Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, userID, enum.AuditUserProfileUpdate, userID, "",
			app.AuditFlag("提交昵称", input.Nickname != nil), app.AuditFlag("提交头像", input.AvatarFileID != nil))
	})

	return err
}

func (s *Service) VerifyPhone(ctx context.Context, userID, code string, useForContact bool) (VerifiedPhoneDTO, error) {
	if strings.TrimSpace(code) == "" || len(code) > 512 {
		return VerifiedPhoneDTO{}, app.Invalid("请重新授权获取微信手机号")
	}

	if s.WechatPhone == nil {
		return VerifiedPhoneDTO{}, &app.Error{
			Status:  http.StatusServiceUnavailable,
			Code:    enum.ErrorUnavailable,
			Message: "微信手机号验证尚未配置",
		}
	}

	number, err := s.WechatPhone.Number(ctx, code)
	if err != nil {
		return VerifiedPhoneDTO{}, err
	}

	if !verifiedPhonePattern.MatchString(number) {
		return VerifiedPhoneDTO{}, app.Invalid("微信返回的手机号格式无效")
	}

	cipher, err := s.Encrypt(number)
	if err != nil {
		return VerifiedPhoneDTO{}, err
	}

	var receipt string
	if useForContact {
		receipt, err = s.issueContactPhoneReceipt(userID, number)
		if err != nil {
			return VerifiedPhoneDTO{}, err
		}
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		_, err := lockedProfile(ctx, tx, userID)
		if err != nil {
			return false, err
		}

		verification, err := tx.UserVerification.Query().
			Where(userverification.UserIDEQ(userID)).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return false, err
		}

		if verification != nil && s.Decrypt(verification.VerifiedPhoneCipher) == number &&
			(!useForContact || s.Decrypt(verification.ContactPhoneCipher) == number) {
			return false, nil
		}

		contact := tx.UserVerification.Create().
			SetUserID(userID).
			SetVerifiedPhoneCipher(cipher)
		if useForContact {
			contact.SetContactPhoneCipher(cipher)
		}

		save := contact.OnConflictColumns(userverification.FieldUserID).
			SetVerifiedPhoneCipher(cipher).
			SetUpdatedAt(s.Now())
		if useForContact {
			save.UpdateContactPhoneCipher()
		}

		if err := save.Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, userID, enum.AuditUserPhoneVerify, userID, "", app.AuditFlag("用作联系电话", useForContact))
	})
	if err != nil {
		return VerifiedPhoneDTO{}, err
	}

	return VerifiedPhoneDTO{
		VerifiedPhone:       MaskPhone(number),
		ContactPhoneReceipt: receipt,
	}, nil
}

func lockedProfile(ctx context.Context, tx *ent.Tx, userID string) (*ent.User, error) {
	member, err := tx.User.Query().
		Where(user.IDEQ(userID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, app.WrapMissing(err)
	}

	return member, nil
}

func MaskPhone(phone string) string {
	if len(phone) < 8 {
		return ""
	}

	return phone[:3] + "****" + phone[len(phone)-4:]
}
