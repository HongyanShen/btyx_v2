package identity

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/ent/session"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) ChangeOperatorPassword(ctx context.Context, id, current, password string) error {
	hash, err := hashOperatorPassword(password)
	if err != nil {
		return err
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		account, err := tx.Operator.Query().
			Where(operator.IDEQ(id), operator.DisabledEQ(false)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if len(current) > 72 || bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(current)) != nil {
			return false, app.Invalid("当前密码不正确")
		}

		if err := tx.Operator.UpdateOne(account).SetPasswordHash(hash).Exec(ctx); err != nil {
			return false, err
		}

		if err := revokeOperatorSessions(ctx, tx, id); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, id, enum.AuditOperatorUpdate, id, "", "本人修改密码")
	})
	return err
}

func revokeOperatorSessions(ctx context.Context, tx *ent.Tx, id string) error {
	_, err := tx.Session.Delete().
		Where(session.UserIDEQ(id), session.RoleEQ(enum.RoleAdmin)).
		Exec(ctx)
	return err
}
