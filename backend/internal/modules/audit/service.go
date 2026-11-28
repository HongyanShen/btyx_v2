package audit

import (
	"context"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/audithttpresponse"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/ent/predicate"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
	"entgo.io/ent/dialect/sql"
)

type Service struct {
	*app.Runtime
	Store objectstore.Gateway
}

type DTO struct {
	HTTPStatus    int                `json:"http_status,omitempty"`
	Platform      enum.AuditPlatform `json:"platform,omitempty"`
	ActorName     string             `json:"actor_name"`
	ID            string             `json:"id"`
	ActorID       string             `json:"actor_id"`
	ActorUsername string             `json:"actor_username"`
	Action        enum.AuditAction   `json:"action"`
	ReferenceID   string             `json:"reference_id"`
	Reason        string             `json:"reason"`
	CreatedAt     time.Time          `json:"created_at"`
}

func (s *Service) Audits(ctx context.Context) ([]DTO, error) {
	return s.auditRows(ctx, s.Client.AuditEvent.Query().Limit(300))
}

const PageSize = 15
const MissingEndpoint = "unrecorded"

type Filter struct {
	ActorName string
	Platform  enum.AuditPlatform
	Endpoint  string
	Start     time.Time
	Before    time.Time
}

type Page struct {
	Rows   []DTO
	Number int
	Total  int
}

func (s *Service) AuditPage(ctx context.Context, filter Filter, page int) (Page, error) {
	if filter.Platform != "" && !filter.Platform.Valid() {
		return Page{}, app.Invalid("无效的操作平台")
	}

	if !filter.Start.IsZero() && !filter.Before.IsZero() && !filter.Start.Before(filter.Before) {
		return Page{}, app.Invalid("结束时间不能早于开始时间")
	}

	query := s.Client.AuditEvent.Query()
	if filter.Platform != "" {
		query.Where(auditevent.Or(
			auditevent.PlatformEQ(filter.Platform),
			auditevent.And(auditevent.PlatformIsNil(), auditevent.EndpointIn(historicalEndpoints[filter.Platform]...)),
		))
	}

	if name := strings.TrimSpace(filter.ActorName); name != "" {
		query.Where(actorNameContains(name))
	}

	if filter.Endpoint == MissingEndpoint {
		query.Where(auditevent.EndpointEQ(""))
	} else if filter.Endpoint != "" {
		query.Where(auditevent.EndpointEQ(filter.Endpoint))
	}

	if !filter.Start.IsZero() {
		query.Where(auditevent.CreatedAtGTE(filter.Start))
	}

	if !filter.Before.IsZero() {
		query.Where(auditevent.CreatedAtLT(filter.Before))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return Page{}, err
	}

	number := min(max(1, page), max(1, (total+PageSize-1)/PageSize))
	rows, err := s.auditRows(ctx, query.Offset((number-1)*PageSize).Limit(PageSize))
	return Page{
		Number: number,
		Total:  total,
		Rows:   rows,
	}, err
}

func actorNameContains(name string) predicate.AuditEvent {
	return func(selector *sql.Selector) {
		operators := sql.Table(operator.Table)
		operatorIDs := sql.Select(operators.C(operator.FieldID)).From(operators).
			Where(sql.ContainsFold(operators.C(operator.FieldUsername), name))
		members := sql.Table(user.Table)
		memberIDs := sql.Select(members.C(user.FieldID)).From(members).
			Where(sql.And(sql.IsFalse(members.C(user.FieldDeleted)), sql.ContainsFold(members.C(user.FieldNickname), name)))
		actor := selector.C(auditevent.FieldActorID)
		matches := []*sql.Predicate{sql.In(actor, operatorIDs), sql.In(actor, memberIDs)}
		for _, id := range []string{"system", "worker", "admin"} {
			if strings.Contains(app.OperatorNames(nil).Label(id), name) {
				matches = append(matches, sql.EQ(actor, id))
			}
		}

		selector.Where(sql.Or(matches...))
	}
}

func (s *Service) AuditEndpoints(ctx context.Context) ([]string, error) {
	return s.Client.AuditEvent.Query().
		Where(auditevent.EndpointNEQ("")).
		GroupBy(auditevent.FieldEndpoint).
		Strings(ctx)
}

func (s *Service) auditRows(ctx context.Context, query *ent.AuditEventQuery) ([]DTO, error) {
	records, err := query.Order(ent.Desc(auditevent.FieldCreatedAt, auditevent.FieldID)).
		Select(auditevent.FieldID, auditevent.FieldCreatedAt, auditevent.FieldAction,
			auditevent.FieldActorID, auditevent.FieldReferenceID, auditevent.FieldReason, auditevent.FieldPlatform, auditevent.FieldEndpoint, auditevent.FieldRequestID).
		All(ctx)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return []DTO{}, nil
	}

	actorIDs := make([]string, 0, len(records))
	var requestIDs []string
	for _, record := range records {
		actorIDs = append(actorIDs, record.ActorID)
		if record.RequestID != "" {
			requestIDs = append(requestIDs, record.RequestID)
		}
	}

	statuses := make(map[string]int)
	if len(requestIDs) > 0 {
		responses, err := s.Client.AuditHTTPResponse.Query().
			Where(audithttpresponse.IDIn(requestIDs...)).
			Select(audithttpresponse.FieldID, audithttpresponse.FieldStatus).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, response := range responses {
			statuses[response.ID] = response.Status
		}
	}

	names, err := app.LookupOperatorNames(ctx, s.Client, actorIDs)
	if err != nil {
		return nil, err
	}

	var memberIDs []string
	for _, id := range actorIDs {
		if _, found := names[id]; !found && id != "system" && id != "worker" && id != "admin" {
			memberIDs = append(memberIDs, id)
		}
	}

	nicknames := make(map[string]string)
	if len(memberIDs) > 0 {
		members, err := s.Client.User.Query().
			Where(user.IDIn(memberIDs...)).
			Select(user.FieldID, user.FieldNickname).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, member := range members {
			nicknames[member.ID] = member.Nickname
		}
	}

	result := make([]DTO, 0, len(records))
	for _, record := range records {
		actorName := names.Label(record.ActorID)
		if nickname := nicknames[record.ActorID]; nickname != "" {
			actorName = nickname
		}

		result = append(result, DTO{
			HTTPStatus:    statuses[record.RequestID],
			Platform:      recordedPlatform(record.Platform, record.Endpoint),
			ActorName:     actorName,
			ID:            record.ID,
			ActorID:       record.ActorID,
			ActorUsername: names[record.ActorID],
			Action:        record.Action,
			ReferenceID:   record.ReferenceID,
			Reason:        record.Reason,
			CreatedAt:     record.CreatedAt,
		})
	}

	return result, nil
}
