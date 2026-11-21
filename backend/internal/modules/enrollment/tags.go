package enrollment

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/content"
	"baituber.cn/yanxuan/v2/ent/marketingtag"
	"baituber.cn/yanxuan/v2/ent/marketingtagdeletion"
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/marketingtags"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
	"github.com/segmentio/ksuid"
)

func (s *Service) MarketingTags(ctx context.Context) ([]MarketingTagDTO, error) {
	tags, err := s.Client.MarketingTag.Query().
		Order(ent.Asc(marketingtag.FieldCreatedAt, marketingtag.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]MarketingTagDTO, 0, len(tags))
	for _, tag := range tags {
		result = append(result, marketingTagDTO(tag))
	}

	return result, nil
}

func (s *Service) CreateMarketingTag(ctx context.Context, actorID, name string) (MarketingTagDTO, error) {
	name, err := marketingTagName(name)
	if err != nil {
		return MarketingTagDTO{}, err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (MarketingTagDTO, error) {
		if err := marketingtags.Lock(ctx, tx); err != nil {
			return MarketingTagDTO{}, err
		}

		key := name
		for {
			retired, err := tx.MarketingTagDeletion.Query().
				Where(marketingtagdeletion.KeyEQ(key)).
				Exist(ctx)
			if err != nil {
				return MarketingTagDTO{}, err
			}

			if !retired {
				inUse, err := tx.MarketingTag.Query().
					Where(marketingtag.KeyEQ(key)).
					Exist(ctx)
				if err != nil {
					return MarketingTagDTO{}, err
				}

				if !inUse {
					break
				}
			}

			key = ksuid.New().String()
		}

		tag, err := tx.MarketingTag.Create().
			SetKey(key).
			SetName(name).
			SetUpdatedAt(s.Now().Truncate(time.Microsecond)).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return MarketingTagDTO{}, app.Conflict("标签名称已被占用")
		}

		if err != nil {
			return MarketingTagDTO{}, err
		}

		return marketingTagDTO(tag), app.Audit(ctx, tx, actorID, enum.AuditMarketingTagCreate, tag.ID, tag.Name)
	})
}

func (s *Service) RenameMarketingTag(ctx context.Context, actorID, tagID, name, previousName string) error {
	name, err := marketingTagName(name)
	if err != nil {
		return err
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		if err := marketingtags.Lock(ctx, tx); err != nil {
			return false, err
		}

		tag, err := tx.MarketingTag.Query().
			Where(marketingtag.IDEQ(tagID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if tag.Name == name {
			return false, nil
		}

		if tag.Name != previousName {
			return false, app.Conflict("标签名称已更新，请刷新后重新操作")
		}

		err = tx.MarketingTag.UpdateOne(tag).SetName(name).Exec(ctx)
		if ent.IsConstraintError(err) {
			return false, app.Conflict("标签名称已被占用")
		}

		if err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditMarketingTagRename, tag.ID, previousName+" → "+name)
	})
	return err
}

func (s *Service) DeleteMarketingTag(ctx context.Context, actorID, tagID string, updatedAt time.Time) error {
	if updatedAt.IsZero() {
		return app.Invalid("请刷新标签库后重新删除")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		if err := marketingtags.Lock(ctx, tx); err != nil {
			return false, err
		}

		tag, err := tx.MarketingTag.Get(ctx, tagID)
		if ent.IsNotFound(err) {
			deleted, err := tx.MarketingTagDeletion.Query().
				Where(marketingtagdeletion.IDEQ(tagID)).
				Exist(ctx)
			if err != nil || deleted {
				return false, err
			}

			return false, app.NotFound()
		}

		if err != nil {
			return false, err
		}

		if !tag.UpdatedAt.Equal(updatedAt) {
			return false, app.Conflict("标签已更新，请刷新标签库后重新删除")
		}

		if err := checkTagUnused(ctx, tx, tag.Key); err != nil {
			return false, err
		}

		if err := tx.MarketingTag.DeleteOne(tag).Exec(ctx); err != nil {
			return false, err
		}

		err = tx.MarketingTagDeletion.Create().
			SetID(tag.ID).
			SetKey(tag.Key).
			SetCreatedAt(s.Now()).
			Exec(ctx)
		if err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditMarketingTagDelete, tag.ID, tag.Name)
	})
	return err
}

func checkTagUnused(ctx context.Context, tx *ent.Tx, key string) error {
	enrollmentUsesTag, err := tx.UserEnrollment.Query().
		Where(
			userenrollment.StatusNEQ(enum.EnrollmentNone),
			userenrollment.HasUserWith(mixin.NotDeleted),
			func(selector *sql.Selector) {
				selector.Where(sqljson.ValueContains(selector.C(userenrollment.FieldTags), key))
			},
		).
		Exist(ctx)
	if err != nil {
		return err
	}

	if enrollmentUsesTag {
		return app.Conflict("标签仍绑定到分发员报名，请先在报名复核中解除绑定")
	}

	contentUsesTag, err := tx.Content.Query().
		Where(func(selector *sql.Selector) {
			selector.Where(sqljson.ValueContains(selector.C(content.FieldTags), key))
		}).
		Exist(ctx)
	if err != nil {
		return err
	}

	if contentUsesTag {
		return app.Conflict("标签仍绑定到内容池素材，请先在内容池解除绑定")
	}

	taskUsesTag, err := tx.Task.Query().
		Where(func(selector *sql.Selector) {
			versions := sql.Table(taskversion.Table)
			matchingVersions := sql.Select(versions.C(taskversion.FieldID)).
				From(versions).
				Where(sqljson.ValueContains(versions.C(taskversion.FieldTags), key))
			selector.Where(sql.In(selector.C(task.FieldCurrentVersionID), matchingVersions))
		}).
		Exist(ctx)
	if err != nil {
		return err
	}

	if taskUsesTag {
		return app.Conflict("标签仍绑定到任务当前版本，请先发布解除绑定的新版本")
	}

	return nil
}

func marketingTagName(name string) (string, error) {
	name = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(name), "#＃"))
	if name == "" || utf8.RuneCountInString(name) > 24 || strings.ContainsAny(name, "#＃") || strings.ContainsFunc(name, func(value rune) bool {
		return unicode.IsControl(value) || unicode.IsSpace(value)
	}) {
		return "", app.Invalid("请填写 1–24 字且不含空格或 # 的标签名称")
	}

	return name, nil
}

func marketingTagDTO(tag *ent.MarketingTag) MarketingTagDTO {
	return MarketingTagDTO{
		ID:        tag.ID,
		Key:       tag.Key,
		Name:      tag.Name,
		UpdatedAt: tag.UpdatedAt,
	}
}
