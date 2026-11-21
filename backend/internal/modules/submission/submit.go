package submission

import (
	"context"
	"slices"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	tasks "baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) eligibleDate(assignmentRecord *ent.Assignment) string {
	z := time.FixedZone("CST", 8*3600)
	d, _ := time.ParseInLocation("2006-01-02", assignmentRecord.BusinessDate, z)
	days := (8 - int(d.Weekday())) % 7
	if days == 0 {
		days = 7
	}

	due := d.AddDate(0, 0, days).
		Add(12 * time.Hour)
	if !s.Now().
		Before(due) {
		now := s.Now().
			In(z)
		days = (8 - int(now.Weekday())) % 7
		if days == 0 {
			days = 7
		}

		return now.AddDate(0, 0, days).
			Format("2006-01-02")
	}

	return due.Format("2006-01-02")
}

func (s *Service) Submit(ctx context.Context, userID, id string, input SubmitInput) error {
	workID, err := workKey(input.Platform, input.URL)
	if err != nil {
		return err
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		assignmentRecord, err := tx.Assignment.Query().
			Where(assignment.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil || assignmentRecord.UserID != userID {
			return false, app.NotFound()
		}

		if assignmentRecord.Status == enum.AssignmentRevoked || assignmentRecord.Status == enum.AssignmentExpired {
			return false, app.Conflict("任务已结束")
		}

		if !slices.Contains(assignmentRecord.Platforms, input.Platform) {
			return false, app.Invalid("此任务不要求该平台")
		}

		old, err := tx.Submission.Query().
			Where(submission.AssignmentIDEQ(id), submission.PlatformEQ(input.Platform)).
			Only(ctx)
		if err == nil {
			if old.Status == enum.SubmissionInvalid {
				return false, app.Conflict("该平台记录已失效，请使用重新派发的任务提交")
			}

			if old.SettlementID != "" {
				return false, app.Conflict("已结算记录不可修改")
			}

			if old.Status == enum.SubmissionApproved || old.Status == enum.SubmissionWaived {
				if old.URL != input.URL {
					return false, app.Conflict("已通过的作品不能替换")
				}

				if old.Screenshot == input.Screenshot {
					return true, nil
				}

				_, err = tx.Submission.UpdateOneID(old.ID).
					SetScreenshot(input.Screenshot).
					SetEligibleDate(s.eligibleDate(assignmentRecord)).
					Save(ctx)
				if err != nil {
					return false, err
				}

				return true, tasks.RefreshAssignment(ctx, tx, id)
			}

			if old.URL == input.URL && old.Screenshot == input.Screenshot && old.Status == enum.SubmissionPending {
				return true, nil
			}

			_, err = tx.Submission.UpdateOneID(old.ID).
				SetURL(input.URL).
				SetWorkKey(workID).
				SetScreenshot(input.Screenshot).
				SetStatus(enum.SubmissionPending).
				SetReason("").
				SetRevision(old.Revision + 1).
				SetEligibleDate(s.eligibleDate(assignmentRecord)).
				Save(ctx)
		} else if ent.IsNotFound(err) {
			if assignmentRecord.ExpiresAt != nil && !s.Now().Before(*assignmentRecord.ExpiresAt) {
				hasLink, err := tx.Submission.Query().
					Where(submission.AssignmentIDEQ(id)).
					Exist(ctx)
				if err != nil {
					return false, err
				}

				if !hasLink {
					return false, app.Conflict("已超过发布链接提交时限")
				}
			}

			if assignmentRecord.ExpiresAt == nil && s.Day() > assignmentRecord.DueDate {
				return false, app.Conflict("已过当天发布期限，请联系运营")
			}

			_, err = tx.Submission.Create().
				SetAssignmentID(id).
				SetPlatform(input.Platform).
				SetURL(input.URL).
				SetWorkKey(workID).
				SetScreenshot(input.Screenshot).
				SetEligibleDate(s.eligibleDate(assignmentRecord)).
				Save(ctx)
		}

		if err != nil {
			return false, err
		}

		if err = app.Audit(ctx, tx, userID, enum.AuditSubmissionSubmit, id, string(input.Platform)); err != nil {
			return false, err
		}

		return true, tasks.RefreshAssignment(ctx, tx, id)
	})
	return err
}
