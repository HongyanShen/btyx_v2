package notification

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type Service struct {
	*app.Runtime
}

type DTO struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	ReferenceID string    `json:"reference_id"`
	Read        bool      `json:"read"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) Notifications(ctx context.Context, userID string) ([]DTO, error) {
	records, err := s.Client.Notification.Query().
		Where(notification.UserIDEQ(userID)).
		Order(ent.Desc(notification.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		result = append(result, DTO{
			ID:          record.ID,
			Title:       record.Title,
			Body:        record.Body,
			ReferenceID: record.ReferenceID,
			Read:        record.Read,
			CreatedAt:   record.CreatedAt,
		})
	}

	return result, nil
}

func (s *Service) ReadNotification(ctx context.Context, userID, id string) error {
	affected, err := s.Client.Notification.Update().
		Where(notification.IDEQ(id), notification.UserIDEQ(userID)).
		SetRead(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if affected == 0 {
		return app.NotFound()
	}

	return nil
}
