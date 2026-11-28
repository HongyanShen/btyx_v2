package feedback

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/feedback"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type Service struct {
	*app.Runtime
}

type DTO struct {
	ID        string              `json:"id"`
	UserID    string              `json:"user_id"`
	Body      string              `json:"body"`
	Reply     string              `json:"reply"`
	Status    enum.FeedbackStatus `json:"status"`
	CreatedAt time.Time           `json:"created_at"`
}

func (s *Service) Feedbacks(ctx context.Context, userID string, role enum.Role) ([]DTO, error) {
	query := s.Client.Feedback.Query()
	if role != enum.RoleAdmin {
		query = query.Where(feedback.UserIDEQ(userID))
	}

	records, err := query.Order(ent.Desc(feedback.FieldCreatedAt)).Limit(200).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		result = append(result, DTO{

			ID:     record.ID,
			UserID: record.UserID,
			Body:   record.Body,

			Reply:     record.Reply,
			Status:    record.Status,
			CreatedAt: record.CreatedAt,
		})
	}

	return result, nil
}

func (s *Service) CreateFeedback(ctx context.Context, userID, body string) error {
	if strings.TrimSpace(body) == "" || len([]rune(body)) > 2000 {
		return app.Invalid("反馈内容应为1至2000字")
	}

	_, err := s.Client.Feedback.Create().SetUserID(userID).SetBody(body).Save(ctx)

	return err
}

func (s *Service) ReplyFeedback(ctx context.Context, actorID, id, reply string) error {
	if strings.TrimSpace(reply) == "" {
		return app.Invalid("回复内容不能为空")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		record, err := tx.Feedback.UpdateOneID(id).
			SetReply(reply).
			SetActorID(actorID).
			SetStatus(enum.FeedbackReplied).
			Save(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		if err := s.Notify(ctx, tx, record.UserID, "反馈已回复", reply, id); err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, actorID, enum.AuditFeedbackReply, id, "", fmt.Sprintf("回复字数：%d", utf8.RuneCountInString(reply)))
	})

	return err
}
