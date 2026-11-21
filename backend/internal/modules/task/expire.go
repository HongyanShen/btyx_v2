package task

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
)

func (s *Service) ExpireMarketingAssignments(ctx context.Context) error {
	now := s.Now()
	ids, err := s.Client.Assignment.Query().
		Where(
			assignment.StatusEQ(enum.AssignmentInProgress),
			assignment.ExpiresAtLTE(now),
			func(row *sql.Selector) {
				link := sql.Table(submission.Table)
				exists := sql.Select(link.C(submission.FieldID)).
					From(link).
					Where(sql.ColumnsEQ(link.C(submission.FieldAssignmentID), row.C(assignment.FieldID)))
				row.Where(sql.Not(sql.Exists(exists)))
			},
		).
		Order(ent.Asc(assignment.FieldExpiresAt)).
		Limit(100).
		IDs(ctx)
	if err != nil {
		return err
	}

	for _, id := range ids {
		_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
			record, err := tx.Assignment.Query().
				Where(assignment.IDEQ(id)).
				ForUpdate().
				Only(ctx)
			if err != nil {
				return false, err
			}

			if record.Status != enum.AssignmentInProgress || record.ExpiresAt == nil || record.ExpiresAt.After(now) {
				return false, nil
			}

			linked, err := tx.Submission.Query().
				Where(submission.AssignmentIDEQ(id)).
				Exist(ctx)
			if err != nil || linked {
				return false, err
			}

			err = tx.Assignment.UpdateOneID(id).
				SetStatus(enum.AssignmentExpired).
				ClearAllocationKey().
				Exec(ctx)
			if err != nil {
				return false, err
			}

			return true, app.Audit(ctx, tx, "system", enum.AuditTaskExpire, id, "到期未提交发布链接")
		})
		if err != nil {
			return err
		}
	}

	return nil
}
