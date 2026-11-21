package task

import (
	"context"
	"slices"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) CompleteStep(ctx context.Context, userID, id string, key enum.TaskStep) error {
	if !slices.Contains([]enum.TaskStep{enum.StepContent, enum.StepPublish}, key) {
		return app.Invalid("该步骤由实际提交或结算自动完成")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		assignmentRecord, err := tx.Assignment.Query().
			Where(assignment.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil || assignmentRecord.UserID != userID {
			return false, app.NotFound()
		}

		if assignmentRecord.Status == enum.AssignmentRevoked {
			return false, app.Conflict("任务已撤回")
		}

		if slices.Contains(assignmentRecord.Steps, key) {
			return true, nil
		}

		_, err = tx.Assignment.UpdateOneID(id).
			SetSteps(append(assignmentRecord.Steps, key)).
			Save(ctx)
		return err == nil, err
	})
	return err
}

func RefreshAssignment(ctx context.Context, tx *ent.Tx, id string) error {
	assignmentRecord, err := tx.Assignment.Query().
		Where(assignment.IDEQ(id)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return err
	}

	rows, err := tx.Submission.Query().
		Where(submission.AssignmentIDEQ(id)).
		All(ctx)
	if err != nil {
		return err
	}

	status := aggregateStatus(assignmentRecord.Platforms, rows)

	_, err = tx.Assignment.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	return err
}

func aggregateStatus(platforms []enum.Platform, submissions []*ent.Submission) enum.AssignmentStatus {
	payable := 0
	settled := 0
	hasRejected := false
	hasPending := false

	for _, record := range submissions {
		switch record.Status {
		case enum.SubmissionRejected:
			hasRejected = true
		case enum.SubmissionPending:
			hasPending = true
		case enum.SubmissionApproved, enum.SubmissionWaived:
			payable++
			if record.SettlementID != "" {
				settled++
			}
		}
	}

	if hasRejected {
		return enum.AssignmentRejected
	}

	if hasPending {
		return enum.AssignmentPendingReview
	}

	if len(submissions) < len(platforms) {
		return enum.AssignmentInProgress
	}

	if payable == 0 {
		return enum.AssignmentNotPayable
	}

	if settled == payable {
		return enum.AssignmentSettled
	}

	return enum.AssignmentPendingSettlement
}
