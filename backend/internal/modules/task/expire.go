package task

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"entgo.io/ent/dialect/sql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *Service) ExpireMarketingAssignments(ctx context.Context) (err error) {
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
	if err != nil || len(ids) == 0 {
		return err
	}

	ctx, span := otel.Tracer("yanxuan/task").Start(ctx, "expire marketing assignments", trace.WithAttributes(attribute.Int("assignment.count", len(ids))))
	defer func() { telemetry.Finish(span, err) }()
	for _, id := range ids {
		_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (_ bool, checkErr error) {
			ctx, checkSpan := otel.Tracer("yanxuan/task").Start(ctx, "check assignment expiration")
			defer func() { telemetry.Finish(checkSpan, checkErr) }()
			record, err := tx.Assignment.Query().
				Where(assignment.IDEQ(id)).
				ForUpdate().
				Only(ctx)
			if err != nil {
				return false, err
			}

			telemetry.SetUser(ctx, record.UserID, enum.RoleUser)
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

			codes, err := taskCodes(ctx, tx.Client(), []string{record.TaskID})
			if err != nil {
				return false, err
			}

			return true, app.Audit(ctx, tx, "system", enum.AuditTaskExpire, id, "到期未提交发布链接",
				"任务编号："+codes[record.TaskID], "首次提交截止："+record.ExpiresAt.Format(time.RFC3339))
		})
		if err != nil {
			return err
		}
	}

	return nil
}
