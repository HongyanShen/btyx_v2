package creative

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"
	"slices"
	"strings"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/creativelink"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/shopspring/decimal"
)

func reward(tiers []Tier, likes int) decimal.Decimal {
	amount := decimal.Zero
	for _, tier := range tiers {
		if likes < tier.Likes {
			break
		}

		amount = decimal.RequireFromString(tier.RewardFen)
	}

	return amount
}

func (s *Service) Evidence(ctx context.Context, uid, id string, in Link) error {
	return s.change(ctx, id, uid, func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		if w.Status != enum.CreativeLinked || !slices.Contains(t.Platforms, in.Platform) {
			return app.Conflict("请在交齐发布链接后补交点赞截图")
		}

		link, err := tx.CreativeLink.Query().
			Where(creativelink.WorkIDEQ(id), creativelink.PlatformEQ(in.Platform)).
			Only(ctx)
		if err != nil {
			return app.WrapMissing(err)
		}

		if link.Screenshot == in.Screenshot && in.Screenshot != "" {
			return nil
		}

		if s.Now().Before(t.RewardAt) || s.Now().Before(app.BeijingTime(link.CreatedAt).AddDate(0, 0, t.PublishDays)) {
			return app.Conflict("尚未到兑奖时间或未满发布天数")
		}

		if err := requireFile(ctx, tx, in.Screenshot, uid, false); err != nil {
			return err
		}

		if err := tx.CreativeLink.UpdateOne(link).
			SetScreenshot(in.Screenshot).
			SetEvidenceReason("").
			Exec(ctx); err != nil {
			return err
		}

		return app.Audit(ctx, tx, uid, enum.AuditCreativeEvidence, id, string(in.Platform), "平台："+string(in.Platform))
	})
}

func (s *Service) Award(ctx context.Context, actor, id string, in LinksInput) error {
	return s.change(ctx, id, "", func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		if w.Status != enum.CreativeLinked && w.Status != enum.CreativeRewarded {
			return app.Conflict("作品尚未提交链接")
		}

		links, err := tx.CreativeLink.Query().Where(creativelink.WorkIDEQ(id)).All(ctx)
		if err != nil {
			return err
		}

		if len(links) != len(t.Platforms) || len(in.Links) != len(links) {
			return app.Invalid("请逐平台核对截图并填写点赞数")
		}

		settings, err := taskDTO(ctx, tx.Client(), t)
		if err != nil {
			return err
		}

		total := decimal.Zero
		sum := 0
		for _, link := range links {
			index := slices.IndexFunc(in.Links, func(input Link) bool { return input.Platform == link.Platform })
			if index < 0 {
				return app.Invalid("缺少平台点赞数")
			}

			input := in.Links[index]
			if input.Likes == nil || *input.Likes < 0 || *input.Likes > 2147483647 {
				return app.Invalid("请填写有效的点赞数")
			}

			if input.Screenshot == "" || input.Screenshot != link.Screenshot {
				return app.Conflict("截图已变化，请刷新后重新核实")
			}

			if w.Status == enum.CreativeRewarded {
				if link.Likes == nil || *input.Likes != *link.Likes {
					return app.Conflict("奖励已经核定")
				}

				continue
			}

			if s.Now().Before(t.RewardAt) || s.Now().Before(app.BeijingTime(link.CreatedAt).AddDate(0, 0, t.PublishDays)) {
				return app.Conflict("未到兑奖时间")
			}

			sum += *input.Likes
			if !t.CombinedLikes {
				total = total.Add(reward(settings.Tiers, *input.Likes))
			}

			if err := tx.CreativeLink.UpdateOne(link).SetLikes(*input.Likes).Exec(ctx); err != nil {
				return err
			}
		}

		if w.Status == enum.CreativeRewarded {
			return nil
		}

		if t.CombinedLikes {
			total = reward(settings.Tiers, sum)
		}

		if err := addEarning(ctx, tx, w, total, true, s.Now()); err != nil {
			return err
		}

		if err := tx.CreativeWork.UpdateOne(w).SetStatus(enum.CreativeRewarded).Exec(ctx); err != nil {
			return err
		}

		if err := s.Notify(ctx, tx, w.UserID, "点赞奖励已核定", t.Title, id, wxnotice.TaskStatus(t.Title, "点赞奖励已核定", "请查看奖励明细", "subpackages/task/creative/index?work="+id)); err != nil {
			return err
		}

		return app.Audit(ctx, tx, actor, enum.AuditCreativeReward, id, "点赞奖励 "+total.String()+" 分", "奖励金额："+total.String()+" 分")
	})
}

func (s *Service) ReturnEvidence(ctx context.Context, actor, id string, in Link) error {
	if strings.TrimSpace(in.Reason) == "" || in.Screenshot == "" {
		return app.Invalid("请指明截图并填写退回原因")
	}

	return s.change(ctx, id, "", func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		if w.Status != enum.CreativeLinked {
			return app.Conflict("该任务奖励已核定或尚未提交链接")
		}

		link, err := tx.CreativeLink.Query().
			Where(creativelink.WorkIDEQ(id), creativelink.PlatformEQ(in.Platform)).
			Only(ctx)
		if err != nil {
			return app.WrapMissing(err)
		}

		if link.Screenshot != in.Screenshot {
			return app.Conflict("截图已更换或退回，请刷新")
		}

		if err := tx.CreativeLink.UpdateOne(link).SetScreenshot("").SetEvidenceReason(in.Reason).Exec(ctx); err != nil {
			return err
		}

		if err := s.Notify(ctx, tx, w.UserID, "点赞截图需补交", in.Reason, id, wxnotice.TaskStatus("创作任务", "点赞截图需补交", in.Reason, "subpackages/task/creative/index?work="+id)); err != nil {
			return err
		}

		return app.Audit(ctx, tx, actor, enum.AuditCreativeReview, id, string(in.Platform)+" 点赞截图退回："+in.Reason, "平台："+string(in.Platform))
	})
}
