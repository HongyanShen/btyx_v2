package task

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"baituber.cn/yanxuan/v2/ent/content"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	contents "baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/marketingtags"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/displaynumber"
	"baituber.cn/yanxuan/v2/internal/platform/money"
)

func versionDTO(version *ent.TaskVersion, code string) VersionDTO {
	return VersionDTO{
		TaskCode:  code,
		ID:        version.ID,
		TaskID:    version.TaskID,
		Number:    version.Number,
		CreatorID: version.CreatorID,
		CreatedAt: version.CreatedAt,
		TaskInput: TaskInput{
			Title:     version.Title,
			Body:      version.Body,
			Project:   version.Project,
			ContentID: version.ContentID,
			Files:     version.Files,
			PriceFen:  version.PriceFen.String(),
			Platforms: version.Platforms,
			Tags:      version.Tags,
			Deadline:  version.Deadline,
		},
	}
}

func validateTask(input TaskInput) error {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" {
		return app.Invalid("标题与发布要求不能为空")
	}

	price, err := money.ParseFen(input.PriceFen)
	if err != nil || !price.IsPositive() {
		return app.Invalid("单价必须是正整数分")
	}

	if len(input.Platforms) < 1 || len(input.Platforms) > 2 {
		return app.Invalid("请选择发布平台")
	}

	for index, platform := range input.Platforms {
		if !platform.Valid() || slices.Contains(input.Platforms[:index], platform) {
			return app.Invalid("发布平台无效或重复")
		}
	}

	if input.Deadline != "" && !app.ValidDate(input.Deadline) {
		return app.Invalid("截止日期格式应为 YYYY-MM-DD")
	}

	return nil
}

func validateVersionReferences(ctx context.Context, tx *ent.Tx, actorID string, input TaskInput) error {
	for _, fileID := range input.Files {
		if err := files.ValidateOperatorReference(ctx, tx, actorID, fileID); err != nil {
			return err
		}
	}

	return marketingtags.CheckBindings(ctx, tx, input.Tags)
}

func writeVersion(ctx context.Context, tx *ent.Tx, actorID, taskID string, versionNumber int64, input TaskInput) (*ent.TaskVersion, error) {
	price, _ := money.ParseFen(input.PriceFen)

	return tx.TaskVersion.Create().
		SetTaskID(taskID).
		SetNumber(versionNumber).
		SetTitle(input.Title).
		SetBody(input.Body).
		SetProject(input.Project).
		SetContentID(input.ContentID).
		SetFiles(input.Files).
		SetTags(input.Tags).
		SetPlatforms(input.Platforms).
		SetPriceFen(price).
		SetDeadline(input.Deadline).
		SetCreatorID(actorID).
		Save(ctx)
}

func (s *Service) CreateTask(ctx context.Context, actorID string, input TaskInput) (VersionDTO, error) {
	number, err := displaynumber.Task.Next(ctx, s.Client)
	if err != nil {
		return VersionDTO{}, err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (VersionDTO, error) {
		if input.PriceFen == "" {
			policy, err := LoadMarketingPolicy(ctx, tx.Client())
			if err != nil {
				return VersionDTO{}, err
			}

			input.PriceFen = policy.PriceFen.String()
		}

		if err := validateTask(input); err != nil {
			return VersionDTO{}, err
		}

		if input.ContentID == "" {
			if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
				return VersionDTO{}, err
			}

			material, err := contents.CreateInTransaction(ctx, tx, actorID, contents.Input{
				Title:   input.Title,
				Body:    input.Body,
				Project: input.Project,
				Files:   input.Files,
			})
			if err != nil {
				return VersionDTO{}, err
			}

			input.ContentID = material.ID
		} else {
			exists, err := tx.Content.Query().
				Where(content.IDEQ(input.ContentID)).
				Exist(ctx)
			if err != nil {
				return VersionDTO{}, err
			}

			if !exists {
				return VersionDTO{}, app.Invalid("内容池素材不存在")
			}

			if err := validateVersionReferences(ctx, tx, actorID, input); err != nil {
				return VersionDTO{}, err
			}
		}

		taskRecord, err := tx.Task.Create().
			SetDisplayNumber(number).
			SetCreatorID(actorID).
			Save(ctx)
		if err != nil {
			return VersionDTO{}, err
		}

		version, err := writeVersion(ctx, tx, actorID, taskRecord.ID, 1, input)
		if err != nil {
			return VersionDTO{}, err
		}

		err = tx.Task.UpdateOneID(taskRecord.ID).
			SetCurrentVersionID(version.ID).
			Exec(ctx)
		if err != nil {
			return VersionDTO{}, err
		}

		code, err := DisplayCode(taskRecord.DisplayNumber)
		if err != nil {
			return VersionDTO{}, err
		}

		result := versionDTO(version, code)
		result.Online = taskRecord.Online
		return result, app.Audit(ctx, tx, actorID, enum.AuditTaskCreate, taskRecord.ID, "",
			fmt.Sprintf("版本：%d；平台：%v；单价：%s 分", version.Number, version.Platforms, version.PriceFen.String()))
	})
}

func (s *Service) PublishVersion(ctx context.Context, actorID, taskID string, input TaskInput) (VersionDTO, error) {
	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (VersionDTO, error) {
		taskRecord, err := tx.Task.Query().
			Where(task.IDEQ(taskID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return VersionDTO{}, app.WrapMissing(err)
		}

		old, err := tx.TaskVersion.Get(ctx, taskRecord.CurrentVersionID)
		if err != nil {
			return VersionDTO{}, err
		}

		if input.ContentID == "" {
			input.ContentID = old.ContentID
		}

		if input.PriceFen == "" {
			policy, err := LoadMarketingPolicy(ctx, tx.Client())
			if err != nil {
				return VersionDTO{}, err
			}

			input.PriceFen = policy.AllocationPrice(old.PriceFen).String()
		}

		if err := validateTask(input); err != nil {
			return VersionDTO{}, err
		}

		if err := validateVersionReferences(ctx, tx, actorID, input); err != nil {
			return VersionDTO{}, err
		}

		version, err := writeVersion(ctx, tx, actorID, taskID, old.Number+1, input)
		if err != nil {
			return VersionDTO{}, err
		}

		if _, err = tx.Task.UpdateOneID(taskID).
			SetCurrentVersionID(version.ID).
			Save(ctx); err != nil {
			return VersionDTO{}, err
		}

		code, err := DisplayCode(taskRecord.DisplayNumber)
		if err != nil {
			return VersionDTO{}, err
		}

		result := versionDTO(version, code)
		result.Online = taskRecord.Online
		return result, app.Audit(ctx, tx, actorID, enum.AuditTaskPublishVersion, version.ID, "",
			fmt.Sprintf("版本：%d；平台：%v；单价：%s 分", version.Number, version.Platforms, version.PriceFen.String()))
	})
}
