package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/ent/session"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) createSession(ctx context.Context, sessions *ent.SessionClient, id string, role enum.Role) (LoginResult, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return LoginResult{}, err
	}

	token := hex.EncodeToString(tokenBytes)
	expiresAt := s.Now().Add(24 * time.Hour)

	_, err := sessions.Create().
		SetTokenHash(app.Hash(token)).
		SetUserID(id).
		SetRole(role).
		SetExpiresAt(expiresAt).
		Save(ctx)

	return LoginResult{
		Token:  token,
		UserID: id,
		Role:   role,
	}, err
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	sessionRecord, err := s.Client.Session.Query().
		Where(
			session.TokenHashEQ(app.Hash(token)),
			session.ExpiresAtGT(s.Now()),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return Principal{}, invalidSession()
	}

	if err != nil {
		return Principal{}, err
	}

	principal := Principal{
		ID:   sessionRecord.UserID,
		Role: sessionRecord.Role,
	}
	if principal.Role == enum.RoleUser {
		exists, err := s.Client.User.Query().
			Where(user.IDEQ(principal.ID)).
			Exist(ctx)
		if err != nil {
			return Principal{}, err
		}

		if !exists {
			return Principal{}, invalidSession()
		}
	}

	if principal.Role == enum.RoleAdmin {
		record, err := s.Client.Operator.Query().
			Where(operator.IDEQ(principal.ID), operator.DisabledEQ(false)).
			Select(
				operator.FieldID,
				operator.FieldUsername,
				operator.FieldSuperAdmin,
				operator.FieldPages,
				operator.FieldEndpoints,
			).
			Only(ctx)
		if ent.IsNotFound(err) {
			return Principal{}, invalidSession()
		}

		if err != nil {
			return Principal{}, err
		}

		principal.Operator = operatorDTO(record)
	}

	return principal, nil
}

func invalidSession() error {
	return &app.Error{
		Status:  http.StatusUnauthorized,
		Code:    enum.ErrorUnauthorized,
		Message: "登录已失效",
	}
}

func (s *Service) LogoutUser(ctx context.Context, token string) error {
	_, err := s.Client.Session.Delete().
		Where(session.TokenHashEQ(app.Hash(token)), session.RoleEQ(enum.RoleUser)).
		Exec(ctx)

	return err
}
