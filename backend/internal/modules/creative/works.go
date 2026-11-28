package creative

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/creativelink"
	"baituber.cn/yanxuan/v2/ent/creativework"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"slices"
	"strings"
	"time"
)

type workChange func(*ent.Tx, *ent.CreativeWork, *ent.CreativeTask) error

func (s *Service) change(ctx context.Context, id, owner string, fn workChange) error {
	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		query := tx.CreativeWork.Query().Where(creativework.IDEQ(id))
		if owner != "" {
			query.Where(creativework.UserIDEQ(owner))
		}

		w, err := query.ForUpdate().Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		t, err := tx.CreativeTask.Get(ctx, w.TaskID)
		if err != nil {
			return false, err
		}

		return true, fn(tx, w, t)
	})
	return err
}

func requireFile(ctx context.Context, tx *ent.Tx, id, uid string, video bool) error {
	row, err := tx.FileObject.Query().
		Where(fileobject.IDEQ(id), fileobject.OwnerIDEQ(uid), fileobject.StatusEQ(enum.FileReady), fileobject.IdentityPrivateEQ(false)).
		Only(ctx)
	if err != nil {
		return app.WrapMissing(err)
	}

	if (row.MediaType == enum.FileMP4) != video {
		return app.Invalid("文件类型与任务要求不符")
	}

	return nil
}

func (s *Service) Submit(ctx context.Context, uid, id string, in SubmitInput) error {
	if strings.TrimSpace(in.CopyText) == "" || len(in.Files) == 0 {
		return app.Invalid("请提交作品和发布文案")
	}

	return s.change(ctx, id, uid, func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		if w.Status == enum.CreativePending && w.Revision == in.Revision+1 && w.CopyText == in.CopyText && slices.Equal(w.Files, in.Files) {
			return nil
		}

		if w.Revision != in.Revision || w.Status != enum.CreativeDraft && w.Status != enum.CreativeRejected {
			return app.Conflict("作品状态已变化，请刷新")
		}

		if !s.Now().Before(t.EndsAt) {
			return app.Conflict("投稿已截止")
		}

		for _, file := range in.Files {
			if err := requireFile(ctx, tx, file, uid, t.MediaType == enum.FileMP4); err != nil {
				return err
			}
		}

		if err := tx.CreativeWork.UpdateOne(w).
			SetCopyText(in.CopyText).
			SetFiles(in.Files).
			SetStatus(enum.CreativePending).
			SetReason("").
			AddRevision(1).
			Exec(ctx); err != nil {
			return err
		}

		return app.Audit(ctx, tx, uid, enum.AuditCreativeSubmit, id, "", fmt.Sprintf("修订号：%d；附件数：%d", in.Revision, len(in.Files)))
	})
}

func (s *Service) Review(ctx context.Context, actor, id string, in ReviewInput) error {
	if !in.Approve && strings.TrimSpace(in.Reason) == "" {
		return app.Invalid("请填写驳回原因")
	}

	return s.change(ctx, id, "", func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		status := enum.CreativeRejected
		message := "作品已驳回"
		if in.Approve {
			status = enum.CreativeApproved
			message = "作品已通过，可发布并提交链接"
		}

		if w.Revision == in.Revision && w.Status == status && w.Reason == in.Reason {
			return nil
		}

		if w.Status != enum.CreativePending || w.Revision != in.Revision {
			return app.Conflict("作品状态已变化，请刷新")
		}

		if err := tx.CreativeWork.UpdateOne(w).
			SetStatus(status).
			SetReviewerID(actor).
			SetReason(in.Reason).
			Exec(ctx); err != nil {
			return err
		}

		card := wxnotice.ReviewCard(t.Title, in.Approve, in.Reason, "subpackages/task/creative/index?work="+id)
		if err := s.Notify(ctx, tx, w.UserID, message, t.Title, id, card); err != nil {
			return err
		}

		return app.Audit(ctx, tx, actor, enum.AuditCreativeReview, id, in.Reason,
			fmt.Sprintf("修订号：%d", in.Revision), app.AuditFlag("通过", in.Approve))
	})
}

func (s *Service) Links(ctx context.Context, uid, id string, in LinksInput) error {
	return s.change(ctx, id, uid, func(tx *ent.Tx, w *ent.CreativeWork, t *ent.CreativeTask) error {
		if w.Status != enum.CreativeApproved && w.Status != enum.CreativeLinked {
			return app.Conflict("审核通过后可提交发布链接")
		}

		if len(in.Links) != len(t.Platforms) {
			return app.Invalid("请交齐任务要求的发布链接")
		}

		seen := make(map[enum.Platform]bool)
		for _, link := range in.Links {
			if seen[link.Platform] || !slices.Contains(t.Platforms, link.Platform) {
				return app.Invalid("请按任务指定的平台提交")
			}

			seen[link.Platform] = true
			if _, err := submission.WorkKey(link.Platform, link.URL); err != nil {
				return err
			}

			if w.Status == enum.CreativeLinked {
				existing, err := tx.CreativeLink.Query().
					Where(creativelink.WorkIDEQ(id), creativelink.PlatformEQ(link.Platform)).
					Only(ctx)
				if err != nil {
					return err
				}

				if existing.URL != link.URL {
					return app.Conflict("发布链接已提交")
				}

				continue
			}

			if !s.Now().Before(t.EndsAt) {
				return app.Conflict("投稿已截止")
			}

			if err := tx.CreativeLink.Create().
				SetWorkID(id).
				SetCreatedAt(s.Now()).
				SetPlatform(link.Platform).
				SetURL(link.URL).
				Exec(ctx); err != nil {
				return err
			}
		}

		if w.Status == enum.CreativeLinked {
			return nil
		}

		amount := t.CreationFen
		for range t.Platforms {
			amount = amount.Add(w.PublishFen)
		}

		if err := addEarning(ctx, tx, w, amount, false, s.Now()); err != nil {
			return err
		}

		if err := tx.CreativeWork.UpdateOne(w).SetStatus(enum.CreativeLinked).Exec(ctx); err != nil {
			return err
		}

		return app.Audit(ctx, tx, uid, enum.AuditCreativeLink, id, "", fmt.Sprintf("平台数：%d", len(in.Links)))
	})
}

func addEarning(ctx context.Context, tx *ent.Tx, w *ent.CreativeWork, amount decimal.Decimal, likes bool, now time.Time) error {
	local := app.BeijingTime(now)
	weekday := (8 - int(local.Weekday())) % 7
	if weekday == 0 {
		weekday = 7
	}

	monday := time.Date(local.Year(), local.Month(), local.Day()+weekday, 0, 0, 0, 0, local.Location())
	return tx.CreativeEarning.Create().
		SetWorkID(w.ID).
		SetUserID(w.UserID).
		SetLikesReward(likes).
		SetAmountFen(amount).
		SetEligibleAt(monday).
		Exec(ctx)
}
