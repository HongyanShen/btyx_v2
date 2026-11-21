package identity

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/wechatidentity"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/segmentio/ksuid"
	"github.com/shopspring/decimal"
	"github.com/sqids/sqids-go"
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
	userID, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		userID, err := userIDForOpenID(ctx, tx.Client(), openid)
		if err == nil {
			return userID, nil
		}

		if !ent.IsNotFound(err) {
			return "", err
		}

		userID = app.NewID()
		nickname, err := defaultNickname(userID)
		if err != nil {
			return "", err
		}

		if err := tx.User.Create().
			SetID(userID).
			SetNickname(nickname).
			Exec(ctx); err != nil {
			return "", err
		}

		err = tx.WechatIdentity.Create().
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
	if errors.Is(err, errOpenIDClaimed) {
		// Roll back the candidate user and use the winning registration.
		userID, err = userIDForOpenID(ctx, s.Client, openid)
	}

	if err != nil {
		return LoginResult{}, err
	}

	return s.createSession(ctx, s.Client.Session, userID, enum.RoleUser)
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

func defaultNickname(userID string) (string, error) {
	id, err := ksuid.Parse(userID)
	if err != nil {
		return "", err
	}

	encoder, err := sqids.New(sqids.Options{MinLength: 4})
	if err != nil {
		return "", err
	}

	// The random payload gives each account a short, stable display suffix.
	random := binary.BigEndian.Uint32(id.Payload())
	suffix, err := encoder.Encode([]uint64{uint64(random)})
	if err != nil {
		return "", err
	}

	return "微信用户" + suffix, nil
}
