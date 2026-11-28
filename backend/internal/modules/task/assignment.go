package task

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/ent/user"
	agreements "baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Claim(ctx context.Context, userID, taskID, key string) (AssignmentDTO, error) {
	return s.assign(ctx, userID, userID, taskID, key, enum.SourceClaim)
}

func (s *Service) Assign(ctx context.Context, actorID, userID, taskID, key string) (AssignmentDTO, error) {
	return s.assign(ctx, actorID, userID, taskID, key, enum.SourceAssigned)
}

func (s *Service) assign(ctx context.Context, actorID, userID, taskID, key string, source enum.AssignmentSource) (AssignmentDTO, error) {
	if err := app.ValidateKey(key); err != nil {
		return AssignmentDTO{}, err
	}

	requestKey := userID + ":" + string(source) + ":" + key
	id, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		_, userErr := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			Only(ctx)
		if userErr != nil && !ent.IsNotFound(userErr) {
			return "", userErr
		}

		old, err := tx.Assignment.Query().
			Where(assignment.RequestKeyEQ(requestKey)).
			Only(ctx)
		if err == nil {
			if taskID != "" && old.TaskID != taskID {
				return "", app.Conflict("幂等键已用于另一任务")
			}

			return old.ID, nil
		}

		if !ent.IsNotFound(err) {
			return "", err
		}

		if userErr != nil {
			return "", app.WrapMissing(userErr)
		}

		enrollment, err := agreements.EligibleEnrollment(ctx, tx, userID)
		if err != nil {
			return "", err
		}

		policy, err := LoadMarketingPolicy(ctx, tx.Client())
		if err != nil {
			return "", err
		}

		if !policy.Online {
			return "", app.Conflict("营销分发已下架")
		}

		day := s.Day()
		quotaKey, err := DailyQuotaSlot(ctx, tx, userID, day, policy.DailyLimit)
		if err != nil {
			return "", err
		}

		if quotaKey == "" {
			return "", app.Conflict("今天已领取或被指派任务，名额已满")
		}

		selectedTaskID := taskID
		var candidate *ent.TaskVersion
		if selectedTaskID == "" && source == enum.SourceClaim {
			candidate, err = bestUnallocatedVersion(ctx, tx, day, enrollment.Tags)
			if err != nil {
				return "", err
			}

			selectedTaskID = candidate.TaskID
		}

		taskRecord, err := tx.Task.Query().
			Where(task.IDEQ(selectedTaskID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if candidate != nil && ent.IsNotFound(err) {
				return "", errPoolCandidateChanged
			}

			return "", app.WrapMissing(err)
		}

		if candidate != nil && (!taskRecord.Online || taskRecord.CurrentVersionID != candidate.ID) {
			return "", errPoolCandidateChanged
		}

		if !taskRecord.Online {
			return "", app.NotFound()
		}

		allocated, err := tx.Assignment.Query().
			Where(assignment.AllocationKeyEQ(selectedTaskID)).
			Exist(ctx)
		if err != nil {
			return "", err
		}

		if allocated {
			if candidate != nil {
				return "", errPoolCandidateChanged
			}

			return "", app.Conflict("该内容已被领取或指派，请刷新内容池")
		}

		version := candidate
		if version == nil {
			version, err = tx.TaskVersion.Get(ctx, taskRecord.CurrentVersionID)
			if err != nil {
				return "", err
			}
		}

		if version.Deadline != "" && version.Deadline < day {
			return "", app.Conflict("任务已截止")
		}

		allocatedAt := s.Now()
		if app.BusinessDay(allocatedAt) != day {
			if candidate != nil {
				return "", errPoolCandidateChanged
			}

			return "", app.Conflict("已跨过北京时间零点，请重试")
		}

		assignmentRecord, err := tx.Assignment.Create().
			SetUserID(userID).
			SetTaskID(selectedTaskID).
			SetVersionID(version.ID).
			SetPriceFen(policy.AllocationPrice(version.PriceFen)).
			SetPlatforms(version.Platforms).
			SetSource(source).
			SetBusinessDate(day).
			SetCreatedAt(allocatedAt).
			SetDueDate(day).
			SetExpiresAt(policy.Deadline(allocatedAt)).
			SetQuotaKey(quotaKey).
			SetAllocationKey(selectedTaskID).
			SetRequestKey(requestKey).
			SetActorID(actorID).
			Save(ctx)
		if err != nil {
			return "", err
		}

		auditAction := enum.AuditTaskClaim
		if source == enum.SourceAssigned {
			auditAction = enum.AuditTaskAssign
		}

		code, err := DisplayCode(taskRecord.DisplayNumber)
		if err != nil {
			return "", err
		}

		if err = app.Audit(ctx, tx, actorID, auditAction, assignmentRecord.ID, "", "任务编号："+code, "分发员编号："+userID); err != nil {
			return "", err
		}

		var cards []wxnotice.Card
		if source == enum.SourceAssigned {
			cards = append(cards, wxnotice.Assignment(version.Title, "运营已指派营销任务", "请在截止前提交发布链接", assignmentRecord.ID, *assignmentRecord.ExpiresAt))
		}

		if err = s.Notify(ctx, tx, userID, "营销任务已分配", version.Title, assignmentRecord.ID, cards...); err != nil {
			return "", err
		}

		return assignmentRecord.ID, nil
	})
	if err != nil {
		return AssignmentDTO{}, err
	}

	return s.Assignment(ctx, userID, id)
}

func (s *Service) Revoke(ctx context.Context, actorID, id, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return app.Invalid("请填写撤回原因")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		assignmentRecord, err := tx.Assignment.Query().
			Where(assignment.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if assignmentRecord.Status == enum.AssignmentRevoked {
			return true, nil
		}

		if assignmentRecord.Source != enum.SourceAssigned {
			return false, app.Conflict("仅工作人员指派的任务可撤回")
		}

		n, err := tx.Submission.Query().
			Where(submission.AssignmentIDEQ(id)).
			Count(ctx)
		if err != nil {
			return false, err
		}

		if n > 0 {
			return false, app.Conflict("已提交链接，不能撤回")
		}

		if assignmentRecord.Status == enum.AssignmentExpired ||
			(assignmentRecord.ExpiresAt != nil && !assignmentRecord.ExpiresAt.After(s.Now())) {
			return false, app.Conflict("任务已过期")
		}

		_, err = tx.Assignment.UpdateOneID(id).
			SetStatus(enum.AssignmentRevoked).
			ClearQuotaKey().
			ClearAllocationKey().
			Save(ctx)
		if err != nil {
			return false, err
		}

		card := wxnotice.TaskStatus("营销分发任务", "指派已撤回", reason, wxnotice.Detail(id))
		if err = s.Notify(ctx, tx, assignmentRecord.UserID, "任务指派已撤回", reason, id, card); err != nil {
			return false, err
		}

		codes, err := taskCodes(ctx, tx.Client(), []string{assignmentRecord.TaskID})
		if err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditTaskRevoke, id, reason,
			"任务编号："+codes[assignmentRecord.TaskID], "分发员编号："+assignmentRecord.UserID)
	})
	return err
}
