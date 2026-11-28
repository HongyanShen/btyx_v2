package task

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) SetOnline(ctx context.Context, actorID, taskID string, online bool) error {
	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		_, err := tx.Task.UpdateOneID(taskID).
			SetOnline(online).
			Save(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditTaskVisibility, taskID, "", app.AuditFlag("上架", online))
	})
	return err
}
