package identity

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/wechatidentity"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/displaynumber"
	"baituber.cn/yanxuan/v2/internal/platform/shortcode"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"github.com/segmentio/ksuid"
	"github.com/shopspring/decimal"
)

var errOpenIDClaimed = errors.New("wechat identity registered concurrently")

func (s *Service) DemoLogin(ctx context.Context, device string) (LoginResult, error) {
	if !s.Config.Demo {
		return LoginResult{}, app.Forbidden("本地演示登录未开启")
	}

	if len(device) < 1 || len(device) > 100 {
		return LoginResult{}, app.Invalid("device 长度无效")
	}

	return s.loginOpenID(ctx, "local:"+device)
}

func (s *Service) loginOpenID(ctx context.Context, openid string) (LoginResult, error) {
	userID, err := userIDForOpenID(ctx, s.Client, openid)
	if ent.IsNotFound(err) {
		userID, err = s.registerOpenID(ctx, openid)
	}

	if errors.Is(err, errOpenIDClaimed) {
		userID, err = userIDForOpenID(ctx, s.Client, openid)
	}

	if err != nil {
		return LoginResult{}, err
	}

	login, err := s.createSession(ctx, s.Client.Session, userID, enum.RoleUser)
	if err == nil {
		telemetry.SetUser(ctx, login.UserID, login.Role)
	}

	return login, err
}

func (s *Service) registerOpenID(ctx context.Context, openid string) (string, error) {
	nickname, err := s.newDefaultNickname(ctx)
	if err != nil {
		return "", err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		userID := app.NewID()
		if err := tx.User.Create().
			SetID(userID).
			SetNickname(nickname).
			Exec(ctx); err != nil {
			return "", err
		}

		err := tx.WechatIdentity.Create().
			SetUserID(userID).
			SetOpenid(openid).
			OnConflictColumns(wechatidentity.FieldOpenid).
			DoNothing().
			Exec(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return "", errOpenIDClaimed
		}

		if err != nil {
			return "", err
		}

		err = tx.Wallet.Create().
			SetUserID(userID).
			SetAvailableFen(decimal.Zero).
			SetFrozenFen(decimal.Zero).
			SetWithdrawnFen(decimal.Zero).
			Exec(ctx)

		return userID, err
	})
}

func userIDForOpenID(ctx context.Context, client *ent.Client, openid string) (string, error) {
	identity, err := client.WechatIdentity.Query().
		Where(wechatidentity.OpenidEQ(openid)).
		WithUser().
		Only(ctx)
	if err != nil {
		return "", err
	}

	// An existing WeChat identity must not enter registration if its user is hidden.
	if identity.Edges.User == nil {
		return "", app.Forbidden("账号不可用")
	}

	return identity.UserID, nil
}

func (s *Service) WechatOpenID(ctx context.Context, userID string) (string, error) {
	identity, err := s.Client.WechatIdentity.Query().
		Where(wechatidentity.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return "", app.WrapMissing(err)
	}

	return identity.Openid, nil
}

func (s *Service) newDefaultNickname(ctx context.Context) (string, error) {
	for {
		number, err := displaynumber.Nickname.Next(ctx, s.Client)
		if err != nil {
			return "", err
		}

		suffix, err := shortcode.Encode(uint64(number))
		if err != nil {
			return "", err
		}

		nickname := "微信用户" + suffix
		taken, err := s.Client.User.Query().Where(user.NicknameEQ(nickname)).Exist(ctx)
		if err != nil {
			return "", err
		}

		if !taken {
			return nickname, nil
		}
	}
}

// Older profiles without a stored nickname keep their original display name.
func legacyDefaultNickname(userID string) (string, error) {
	id, err := ksuid.Parse(userID)
	if err != nil {
		return "", err
	}

	// The random payload gives each account a short, stable display suffix.
	random := binary.BigEndian.Uint32(id.Payload())
	suffix, err := shortcode.Encode(uint64(random))
	if err != nil {
		return "", err
	}

	return "微信用户" + suffix, nil
}
