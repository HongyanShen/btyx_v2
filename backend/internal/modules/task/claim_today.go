package task

import (
	"context"
	"errors"
	"slices"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
)

var errPoolCandidateChanged = errors.New("content pool candidate changed")

// DailyQuotaSlot runs after the caller locks the recipient's user row.
func DailyQuotaSlot(ctx context.Context, tx *ent.Tx, userID, day string, limit int) (string, error) {
	used, err := dailyQuotaUsed(ctx, tx.Client(), userID, day)
	if err != nil || used >= limit {
		return "", err
	}

	if used == 0 {
		return userID + ":" + day, nil
	}

	return userID + ":" + day + ":" + app.NewID(), nil
}

func dailyQuotaUsed(ctx context.Context, client *ent.Client, userID, day string) (int, error) {
	return client.Assignment.Query().Where(assignment.UserIDEQ(userID), assignment.BusinessDateEQ(day), assignment.QuotaKeyNotNil()).Count(ctx)
}

func (s *Service) TodayAvailability(ctx context.Context) (TodayAvailabilityDTO, error) {
	return s.UserTodayAvailability(ctx, "")
}

func (s *Service) UserTodayAvailability(ctx context.Context, userID string) (TodayAvailabilityDTO, error) {
	policy, err := LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return TodayAvailabilityDTO{}, err
	}

	result := TodayAvailabilityDTO{
		DailyLimit:      policy.DailyLimit,
		CreativeLimit:   policy.CreativeLimit,
		CreativeOnline:  policy.CreativeOnline,
		PriceFen:        policy.PriceFen.String(),
		SettlementLabel: policy.Schedule.SettlementLabel(),
	}
	if userID != "" {
		result.Used, err = dailyQuotaUsed(ctx, s.Client, userID, s.Day())
		if err != nil {
			return TodayAvailabilityDTO{}, err
		}
	}

	if !policy.Online {
		return result, nil
	}

	available, err := unallocatedVersions(s.Client, s.Day()).Exist(ctx)
	result.HasContent = available
	return result, err
}

func (s *Service) ClaimToday(ctx context.Context, userID, key string) (AssignmentDTO, error) {
	for {
		assigned, err := s.assign(ctx, userID, userID, "", key, enum.SourceClaim)
		if !errors.Is(err, errPoolCandidateChanged) {
			return assigned, err
		}

		// Each attempt locks at most one task. Rank again after another
		// transaction assigns it or changes the version we matched against.
		if err := ctx.Err(); err != nil {
			return AssignmentDTO{}, err
		}
	}
}

func unallocatedVersions(client *ent.Client, day string) *ent.TaskVersionQuery {
	return client.TaskVersion.Query().
		Where(
			taskversion.Or(taskversion.DeadlineEQ(""), taskversion.DeadlineGTE(day)),
			func(version *sql.Selector) {
				currentTask := sql.Table(task.Table)
				active := sql.Select(currentTask.C(task.FieldID)).
					From(currentTask).
					Where(sql.And(
						sql.ColumnsEQ(currentTask.C(task.FieldCurrentVersionID), version.C(taskversion.FieldID)),
						sql.EQ(currentTask.C(task.FieldOnline), true),
					))

				mixin.NotDeleted(active)

				allocation := sql.Table(assignment.Table)
				taken := sql.Select(allocation.C(assignment.FieldID)).
					From(allocation).
					Where(sql.ColumnsEQ(allocation.C(assignment.FieldAllocationKey), version.C(taskversion.FieldTaskID)))

				version.Where(sql.And(sql.Exists(active), sql.Not(sql.Exists(taken))))
			},
		)
}

func bestUnallocatedVersion(ctx context.Context, tx *ent.Tx, day string, tags []string) (*ent.TaskVersion, error) {
	versions, err := unallocatedVersions(tx.Client(), day).
		Order(ent.Asc(taskversion.FieldCreatedAt), ent.Asc(taskversion.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, app.Conflict("暂无可领取任务")
	}

	var best *ent.TaskVersion
	bestScore := -1
	for _, version := range versions {
		score := 0
		for _, tag := range version.Tags {
			if slices.Contains(tags, tag) {
				score++
			}
		}

		if score > bestScore || (score > 0 && score == bestScore) {
			best, bestScore = version, score
		}
	}

	return best, nil
}
