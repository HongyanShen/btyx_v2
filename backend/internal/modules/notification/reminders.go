package notification

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
)

const linkReminderTitle = "发布链接提交提醒"
const playbackReminderTitle = "播放数据提交提醒"

func (s *Service) CreateReminders(ctx context.Context) error {
	policy, err := task.LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return err
	}

	cursor := ""
	for {
		records, err := s.Client.Assignment.Query().
			Where(assignment.IDGT(cursor), assignment.StatusIn(enum.AssignmentInProgress, enum.AssignmentPendingReview, enum.AssignmentRejected, enum.AssignmentPendingSettlement)).
			Order(ent.Asc(assignment.FieldID)).
			Limit(200).
			All(ctx)
		if err != nil {
			return err
		}

		for _, record := range records {
			for _, title := range []string{linkReminderTitle, playbackReminderTitle} {
				if inWindow(record, policy, s.Now(), title) {
					if err = s.createReminder(ctx, record.ID, title, policy); err != nil {
						return err
					}
				}
			}

			cursor = record.ID
		}

		if len(records) < 200 {
			return nil
		}
	}
}

func inWindow(a *ent.Assignment, policy task.MarketingPolicy, now time.Time, title string) bool {
	if title == linkReminderTitle {
		return a.ExpiresAt != nil && now.Before(*a.ExpiresAt) && !now.Before(a.ExpiresAt.Add(-15*time.Minute))
	}

	cutoff := policy.Schedule.PlaybackDeadline(a.CreatedAt)
	return now.Before(cutoff) && !now.Before(cutoff.Add(-2*time.Hour))
}

func reminderNeeded(ctx context.Context, c *ent.Client, a *ent.Assignment, title string) (bool, error) {
	if a.Status == enum.AssignmentRevoked || a.Status == enum.AssignmentSettled || a.Status == enum.AssignmentExpired || a.Status == enum.AssignmentNotPayable {
		return false, nil
	}

	records := c.Submission.Query().Where(submission.AssignmentIDEQ(a.ID))
	if title == linkReminderTitle {
		exists, err := records.Exist(ctx)
		return !exists, err
	}

	return records.Where(submission.ScreenshotEQ(""), submission.StatusIn(enum.SubmissionApproved, enum.SubmissionWaived), submission.SettlementIDEQ("")).
		Exist(ctx)
}

func (s *Service) createReminder(ctx context.Context, id, title string, policy task.MarketingPolicy) error {
	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		a, err := tx.Assignment.Query().Where(assignment.IDEQ(id)).ForUpdate().Only(ctx)
		if err != nil {
			return false, err
		}

		exists, err := tx.Notification.Query().
			Where(notification.UserIDEQ(a.UserID), notification.ReferenceIDEQ(id), notification.TitleEQ(title)).
			Exist(ctx)
		if err != nil || exists {
			return false, err
		}

		needed, err := reminderNeeded(ctx, tx.Client(), a, title)
		if err != nil || !needed {
			return false, err
		}

		version, err := tx.TaskVersion.Get(ctx, a.VersionID)
		if err != nil {
			return false, err
		}

		card := wxnotice.Playback(version.Title, policy.Schedule.PlaybackDeadline(a.CreatedAt))
		if title == linkReminderTitle {
			card = wxnotice.LinkReminder(version.Title, id, *a.ExpiresAt)
		}

		return true, s.Notify(ctx, tx, a.UserID, title, version.Title, id, card)
	})
	return err
}

func (s *Service) ReminderCurrent(ctx context.Context, n *ent.Notification) (bool, error) {
	if n.Title != linkReminderTitle && n.Title != playbackReminderTitle && n.Card.TemplateID != wxnotice.NewTask {
		return true, nil
	}

	a, err := s.Client.Assignment.Get(ctx, n.ReferenceID)
	if err != nil {
		return false, err
	}

	if n.Card.TemplateID == wxnotice.NewTask || n.Title == linkReminderTitle {
		if a.ExpiresAt == nil || !s.Now().Before(*a.ExpiresAt) {
			return false, nil
		}

		return reminderNeeded(ctx, s.Client, a, linkReminderTitle)
	}

	policy, err := task.LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return false, err
	}

	if !inWindow(a, policy, s.Now(), playbackReminderTitle) || n.Card.Data["thing10"] != wxnotice.Playback("", policy.Schedule.PlaybackDeadline(a.CreatedAt)).Data["thing10"] {
		return false, nil
	}

	return reminderNeeded(ctx, s.Client, a, n.Title)
}
