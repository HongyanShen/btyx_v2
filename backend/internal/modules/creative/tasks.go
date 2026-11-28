package creative

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/creativetask"
	"baituber.cn/yanxuan/v2/ent/creativetier"
	"baituber.cn/yanxuan/v2/ent/creativework"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/displaynumber"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"context"
	"fmt"
	"slices"
	"strings"
)

type Service struct{ *app.Runtime }

func (s *Service) Create(ctx context.Context, actor string, in TaskInput) (string, error) {
	fee, err := money.ParseFen(in.CreationFen)
	if err != nil || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Requirements) == "" || !in.MediaType.Valid() {
		return "", app.Invalid("请填写创作标题、要求、作品类型和创作费")
	}

	if len(in.Platforms) == 0 || len(in.Platforms) > 2 || in.PublishDays < 1 || in.PublishDays > 3660 {
		return "", app.Invalid("请选择发布平台和有效的发布天数")
	}

	for i, p := range in.Platforms {
		if !p.Valid() || slices.Contains(in.Platforms[:i], p) {
			return "", app.Invalid("发布平台无效或重复")
		}
	}

	if in.StartsAt.IsZero() || !in.EndsAt.After(in.StartsAt) || in.RewardAt.Before(in.EndsAt) {
		return "", app.Invalid("请检查投稿和兑奖时间")
	}

	last := 0
	for _, tier := range in.Tiers {
		amount, err := money.ParseFen(tier.RewardFen)
		if err != nil || !amount.IsPositive() || tier.Likes <= last {
			return "", app.Invalid("点赞门槛须递增，奖励须大于零")
		}

		last = tier.Likes
	}

	if last == 0 {
		return "", app.Invalid("至少设置一个点赞奖励档位")
	}

	number, err := displaynumber.Task.Next(ctx, s.Client)
	if err != nil {
		return "", err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		for _, id := range in.Files {
			if err := files.ValidateOperatorReference(ctx, tx, actor, id); err != nil {
				return "", err
			}
		}

		policy, err := task.LoadMarketingPolicy(ctx, tx.Client())
		if err != nil {
			return "", err
		}

		record, err := tx.CreativeTask.Create().
			SetDisplayNumber(number).
			SetCreatorID(actor).
			SetTitle(in.Title).
			SetRequirements(in.Requirements).
			SetCopyText(in.CopyText).
			SetTopics(in.Topics).
			SetFiles(in.Files).
			SetMediaType(in.MediaType).
			SetPlatforms(in.Platforms).
			SetCreationFen(fee).
			SetPublishFen(policy.PriceFen).
			SetPublishDays(in.PublishDays).
			SetCombinedLikes(in.CombinedLikes).
			SetStartsAt(in.StartsAt).
			SetEndsAt(in.EndsAt).
			SetRewardAt(in.RewardAt).
			Save(ctx)
		if err != nil {
			return "", err
		}

		for _, tier := range in.Tiers {
			amount, _ := money.ParseFen(tier.RewardFen)
			if err := tx.CreativeTier.Create().
				SetTaskID(record.ID).
				SetLikes(tier.Likes).
				SetRewardFen(amount).
				Exec(ctx); err != nil {
				return "", err
			}
		}

		return record.ID, app.Audit(ctx, tx, actor, enum.AuditCreativeCreate, record.ID, "",
			fmt.Sprintf("创作费：%s 分；发布平台：%v；发布天数：%d", record.CreationFen.String(), in.Platforms, in.PublishDays))
	})
}

func (s *Service) Visibility(ctx context.Context, actor, id string, online bool) error {
	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		row, err := tx.CreativeTask.Query().Where(creativetask.IDEQ(id)).ForUpdate().Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if row.Online == online {
			return true, nil
		}

		if err := tx.CreativeTask.UpdateOne(row).SetOnline(online).Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actor, enum.AuditCreativeVisibility, id, "", app.AuditFlag("上架", online))
	})
	return err
}

func (s *Service) Claim(ctx context.Context, uid, id, key string) (string, error) {
	if err := app.ValidateKey(key); err != nil {
		return "", err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (string, error) {
		if _, err := tx.User.Query().Where(user.IDEQ(uid)).ForUpdate().Only(ctx); err != nil {
			return "", app.WrapMissing(err)
		}

		requestKey := uid + ":" + key
		old, err := tx.CreativeWork.Query().
			Where(creativework.Or(creativework.RequestKeyEQ(requestKey), creativework.And(creativework.UserIDEQ(uid), creativework.TaskIDEQ(id)))).
			First(ctx)
		if err == nil {
			if old.TaskID != id {
				return "", app.Conflict("幂等键已用于其他创作任务")
			}

			return old.ID, nil
		}

		if !ent.IsNotFound(err) {
			return "", err
		}

		if err := agreement.CheckEligibility(ctx, tx, uid); err != nil {
			return "", err
		}

		policy, err := task.LoadMarketingPolicy(ctx, tx.Client())
		if err != nil {
			return "", err
		}

		row, err := tx.CreativeTask.Query().Where(creativetask.IDEQ(id)).ForUpdate().Only(ctx)
		if err != nil {
			return "", app.WrapMissing(err)
		}

		now := s.Now()
		day := app.BusinessDay(now)
		if !policy.CreativeOnline || !row.Online || now.Before(row.StartsAt) || !now.Before(row.EndsAt) {
			return "", app.Conflict("创作任务暂不可领取")
		}

		used, err := tx.CreativeWork.Query().
			Where(creativework.UserIDEQ(uid), creativework.BusinessDateEQ(day)).
			Count(ctx)
		if err != nil {
			return "", err
		}

		if used >= policy.CreativeLimit {
			return "", app.Conflict("今天的创作名额已领满")
		}

		w, err := tx.CreativeWork.Create().
			SetUserID(uid).
			SetTaskID(id).
			SetBusinessDate(day).
			SetPublishFen(policy.PriceFen).
			SetRequestKey(requestKey).
			Save(ctx)
		if err != nil {
			return "", err
		}

		code, err := task.DisplayCode(row.DisplayNumber)
		if err != nil {
			return "", err
		}

		return w.ID, app.Audit(ctx, tx, uid, enum.AuditCreativeClaim, w.ID, "", "任务编号："+code)
	})
}

func taskDTO(ctx context.Context, c *ent.Client, t *ent.CreativeTask) (TaskDTO, error) {
	tiers, err := c.CreativeTier.Query().
		Where(creativetier.TaskIDEQ(t.ID)).
		Order(ent.Asc(creativetier.FieldLikes)).
		All(ctx)
	if err != nil {
		return TaskDTO{}, err
	}

	code, err := task.DisplayCode(t.DisplayNumber)
	if err != nil {
		return TaskDTO{}, err
	}

	out := TaskDTO{
		TaskCode:   code,
		ID:         t.ID,
		Online:     t.Online,
		PublishFen: t.PublishFen.String(),
		TaskInput: TaskInput{
			Title:         t.Title,
			Requirements:  t.Requirements,
			CopyText:      t.CopyText,
			Topics:        t.Topics,
			Files:         t.Files,
			Platforms:     t.Platforms,
			MediaType:     t.MediaType,
			CreationFen:   t.CreationFen.String(),
			PublishDays:   t.PublishDays,
			StartsAt:      t.StartsAt,
			EndsAt:        t.EndsAt,
			RewardAt:      t.RewardAt,
			CombinedLikes: t.CombinedLikes,
		},
	}
	for _, tier := range tiers {
		out.Tiers = append(out.Tiers, Tier{
			Likes:     tier.Likes,
			RewardFen: tier.RewardFen.String(),
		})
	}

	return out, nil
}
