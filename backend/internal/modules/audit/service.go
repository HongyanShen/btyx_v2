package audit

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type Service struct {
	*app.Runtime
	Store objectstore.Gateway
}

type DTO struct {
	ID            string           `json:"id"`
	ActorID       string           `json:"actor_id"`
	ActorUsername string           `json:"actor_username"`
	Action        enum.AuditAction `json:"action"`
	ReferenceID   string           `json:"reference_id"`
	Reason        string           `json:"reason"`
	CreatedAt     time.Time        `json:"created_at"`
}

func (s *Service) Audits(ctx context.Context) ([]DTO, error) {
	records, err := s.Client.AuditEvent.Query().
		Order(ent.Desc(auditevent.FieldCreatedAt)).
		Limit(300).
		All(ctx)
	if err != nil {
		return nil, err
	}

	actorIDs := make([]string, 0, len(records))
	for _, record := range records {
		actorIDs = append(actorIDs, record.ActorID)
	}

	operators, err := s.Client.Operator.Query().
		Where(operator.IDIn(actorIDs...)).
		Select(operator.FieldID, operator.FieldUsername).
		All(ctx)
	if err != nil {
		return nil, err
	}

	usernames := make(map[string]string, len(operators))
	for _, account := range operators {
		usernames[account.ID] = account.Username
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		result = append(result, DTO{
			ID:            record.ID,
			ActorID:       record.ActorID,
			ActorUsername: usernames[record.ActorID],
			Action:        record.Action,
			ReferenceID:   record.ReferenceID,
			Reason:        record.Reason,
			CreatedAt:     record.CreatedAt,
		})
	}

	return result, nil
}
