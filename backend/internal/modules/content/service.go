package content

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/content"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/marketingtags"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type Service struct {
	*app.Runtime
}

type Input struct {
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Project string   `json:"project"`
	Files   []string `json:"files"`
	Tags    []string `json:"tags"`
}

type TagsInput struct {
	Tags         []string `json:"tags"`
	PreviousTags []string `json:"previous_tags"`
}

type DTO struct {
	ID        string    `json:"id"`
	CreatorID string    `json:"creator_id"`
	CreatedAt time.Time `json:"created_at"`
	Input
}

func (s *Service) CreateContent(ctx context.Context, actorID string, input Input) (DTO, error) {
	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (DTO, error) {
		record, err := CreateInTransaction(ctx, tx, actorID, input)
		if err != nil {
			return DTO{}, err
		}

		return toDTO(record), nil
	})
}

func CreateInTransaction(ctx context.Context, tx *ent.Tx, actorID string, input Input) (*ent.Content, error) {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" {
		return nil, app.Invalid("素材标题和发布要求不能为空")
	}

	for _, fileID := range input.Files {
		if err := files.ValidateOperatorReference(ctx, tx, actorID, fileID); err != nil {
			return nil, err
		}
	}

	if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
		return nil, err
	}

	record, err := tx.Content.Create().
		SetTitle(input.Title).
		SetBody(input.Body).
		SetProject(input.Project).
		SetFiles(input.Files).
		SetTags(input.Tags).
		SetCreatorID(actorID).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return record, app.Audit(ctx, tx, actorID, enum.AuditContentCreate, record.ID, "", fmt.Sprintf("附件数：%d；标签：%v", len(input.Files), input.Tags))
}

func (s *Service) UpdateContentTags(ctx context.Context, actorID, contentID string, input TagsInput) error {
	if input.Tags == nil {
		return app.Invalid("请提交标签列表")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		record, err := tx.Content.Query().
			Where(content.IDEQ(contentID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if slices.Equal(record.Tags, input.Tags) {
			return false, nil
		}

		if !slices.Equal(record.Tags, input.PreviousTags) {
			return false, app.Conflict("素材标签已更新，请刷新后重新编辑")
		}

		if err := marketingtags.CheckBindings(ctx, tx, input.Tags); err != nil {
			return false, err
		}

		if err := tx.Content.UpdateOne(record).SetTags(input.Tags).Exec(ctx); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditContentTags, contentID, "", fmt.Sprintf("标签：%v", input.Tags))
	})
	return err
}

func (s *Service) Contents(ctx context.Context) ([]DTO, error) {
	records, err := s.Client.Content.Query().
		Order(ent.Desc(content.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		result = append(result, toDTO(record))
	}

	return result, nil
}

func toDTO(record *ent.Content) DTO {
	return DTO{
		ID:        record.ID,
		CreatorID: record.CreatorID,
		CreatedAt: record.CreatedAt,
		Input: Input{
			Title:   record.Title,
			Body:    record.Body,
			Project: record.Project,
			Files:   record.Files,
			Tags:    record.Tags,
		},
	}
}
