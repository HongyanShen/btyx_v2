package agreement

import (
	"context"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/agreement"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type DTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	BodyHTML  string    `json:"body_html,omitempty"`
	CreatorID string    `json:"creator_id"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) Agreements(ctx context.Context) ([]DTO, error) {
	records, err := s.Client.Agreement.Query().
		Order(ent.Desc(agreement.FieldCreatedAt)).
		Limit(100).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		result = append(result, DTO{

			ID:       record.ID,
			Title:    record.Title,
			Body:     record.Body,
			BodyHTML: record.BodyHTML,

			CreatorID: record.CreatorID,
			Active:    record.Active,
			CreatedAt: record.CreatedAt,
		})
	}

	return result, nil
}

func (s *Service) PublishAgreement(ctx context.Context, actorID, title, body string) error {
	return s.publish(ctx, actorID, title, body, "")
}

func (s *Service) PublishRichAgreement(ctx context.Context, actorID, title, markup string) error {
	body, rich, err := agreementBody(markup)
	if err != nil {
		return err
	}

	return s.publish(ctx, actorID, title, body, rich)
}

func (s *Service) publish(ctx context.Context, actorID, title, body, rich string) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return app.Invalid("协议标题和正文不能为空")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		// There may be no current agreement to row-lock on the first publish.
		if _, err := tx.ExecContext(ctx, `LOCK TABLE agreements IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return false, err
		}

		_, err := tx.Agreement.Update().
			Where(agreement.ActiveEQ(true)).
			SetActive(false).
			Save(ctx)
		if err != nil {
			return false, err
		}

		record, err := tx.Agreement.Create().
			SetTitle(title).
			SetBody(body).
			SetBodyHTML(rich).
			SetCreatorID(actorID).
			Save(ctx)
		if err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditAgreementPublish, record.ID, "")
	})

	return err
}
