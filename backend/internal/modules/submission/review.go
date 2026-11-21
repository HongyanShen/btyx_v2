package submission

import (
	"context"
	"strings"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	tasks "baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) ReviewAtRevision(ctx context.Context, actorID, id string, input ReviewInput) error {
	targets := []ReviewRevision{
		{
			Platform: input.Platform,
			Revision: input.Revision,
		},
	}
	return s.reviewAtRevisions(ctx, actorID, id, targets, input.Approved, input.Reason)
}

func (s *Service) RejectAtRevisions(ctx context.Context, actorID, id string, targets []ReviewRevision, reason string) error {
	return s.reviewAtRevisions(ctx, actorID, id, targets, false, reason)
}

func (s *Service) reviewAtRevisions(ctx context.Context, actorID, id string, targets []ReviewRevision, approved bool, reason string) error {
	if len(targets) == 0 {
		return app.Invalid("请选择发布平台")
	}

	seen := make(map[enum.Platform]bool)
	for _, target := range targets {
		if target.Revision < 1 {
			return app.Invalid("请提供待审核的提交版本")
		}

		if !target.Platform.Valid() {
			return app.Invalid("发布平台无效")
		}

		if seen[target.Platform] {
			return app.Invalid("发布平台不能重复")
		}

		seen[target.Platform] = true
	}

	if !approved && strings.TrimSpace(reason) == "" {
		return app.Invalid("驳回必须填写原因")
	}

	status := enum.SubmissionRejected
	auditAction := enum.AuditSubmissionRejected
	if approved {
		status = enum.SubmissionApproved
		auditAction = enum.AuditSubmissionApproved
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		assignmentRecord, err := tx.Assignment.Query().
			Where(assignment.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		changed := false
		for _, target := range targets {
			record, err := tx.Submission.Query().
				Where(
					submission.AssignmentIDEQ(id),
					submission.PlatformEQ(target.Platform),
				).
				Only(ctx)
			if err != nil {
				return false, app.WrapMissing(err)
			}

			if record.Revision != target.Revision {
				return false, app.Conflict("分发员已更新提交，请刷新后重新审核")
			}

			if record.Status == status && record.Reason == reason {
				continue
			}

			if record.Status != enum.SubmissionPending {
				return false, app.Conflict("该平台记录已被处理，请刷新")
			}

			update := tx.Submission.UpdateOneID(record.ID).
				SetStatus(status).
				SetReason(reason).
				SetReviewerID(actorID)
			if approved {
				update.SetApprovedAt(s.Now()).
					SetVisibility(enum.VisibilityManualVerified)
			}

			if err := update.Exec(ctx); err != nil {
				return false, err
			}

			message := string(target.Platform) + "：" + string(status) + " " + reason
			if err := app.Notify(ctx, tx, assignmentRecord.UserID, "发布审核结果", message, id); err != nil {
				return false, err
			}

			if err := app.Audit(ctx, tx, actorID, auditAction, record.ID, reason); err != nil {
				return false, err
			}

			changed = true
		}

		if changed {
			return true, tasks.RefreshAssignment(ctx, tx, id)
		}

		return true, nil
	})
	return err
}
