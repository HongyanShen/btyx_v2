package enrollment

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Block(ctx context.Context, actorID, userID string, blocked bool, reason string, expectedUpdate *time.Time) error {
	reason = strings.TrimSpace(reason)
	if blocked && reason == "" {
		return app.Invalid("请填写拉黑原因")
	}

	if utf8.RuneCountInString(reason) > 300 {
		return app.Invalid("操作原因最多 300 字")
	}

	if !blocked && reason == "" {
		reason = "取消拉黑"
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

		application := enrollmentstate.Current(member)
		if application == nil || application.Status != enum.EnrollmentApproved {
			return false, app.Conflict("仅可调整已通过报名的分发员")
		}

		if member.Blocked == blocked {
			return false, nil
		}

		if expectedUpdate != nil && !expectedUpdate.Equal(member.UpdatedAt) {
			return false, app.Conflict("分发员资料已更新，请刷新后重试")
		}

		if err := tx.User.UpdateOne(member).SetBlocked(blocked).Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditUserBlock, userID, reason, app.AuditFlag("拉黑", blocked))
	})
	return err
}
