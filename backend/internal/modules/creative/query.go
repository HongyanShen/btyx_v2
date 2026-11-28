package creative

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/creativeearning"
	"baituber.cn/yanxuan/v2/ent/creativelink"
	"baituber.cn/yanxuan/v2/ent/creativetask"
	"baituber.cn/yanxuan/v2/ent/creativework"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"context"
)

func (s *Service) Tasks(ctx context.Context, active bool) ([]TaskDTO, error) {
	query := s.Client.CreativeTask.Query()
	if active {
		now := s.Now()
		query.Where(creativetask.OnlineEQ(true), creativetask.StartsAtLTE(now), creativetask.EndsAtGT(now))
	}

	rows, err := query.Order(ent.Desc(creativetask.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]TaskDTO, 0, len(rows))
	for _, r := range rows {
		dto, err := taskDTO(ctx, s.Client, r)
		if err != nil {
			return nil, err
		}

		out = append(out, dto)
	}

	return out, nil
}

func (s *Service) Works(ctx context.Context, userID string) ([]WorkDTO, error) {
	query := s.Client.CreativeWork.Query()
	if userID != "" {
		query.Where(creativework.UserIDEQ(userID))
	}

	rows, err := query.Order(ent.Desc(creativework.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]WorkDTO, 0, len(rows))
	for _, r := range rows {
		t, err := s.Client.CreativeTask.Get(ctx, r.TaskID)
		if err != nil {
			return nil, err
		}

		td, err := taskDTO(ctx, s.Client, t)
		if err != nil {
			return nil, err
		}

		td.PublishFen = r.PublishFen.String()
		w := WorkDTO{
			ID:           r.ID,
			UserID:       r.UserID,
			Task:         td,
			BusinessDate: r.BusinessDate,
			Status:       r.Status,
			Revision:     r.Revision,
			CopyText:     r.CopyText,
			Files:        r.Files,
			Reason:       r.Reason,
		}
		links, err := s.Client.CreativeLink.Query().
			Where(creativelink.WorkIDEQ(r.ID)).
			Order(ent.Asc(creativelink.FieldPlatform)).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, link := range links {
			w.Links = append(w.Links, Link{
				Platform:   link.Platform,
				URL:        link.URL,
				Screenshot: link.Screenshot,
				Reason:     link.EvidenceReason,
				Likes:      link.Likes,
			})
		}

		earnings, err := s.Client.CreativeEarning.Query().
			Where(creativeearning.WorkIDEQ(r.ID)).
			Order(ent.Asc(creativeearning.FieldCreatedAt)).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, e := range earnings {
			w.Earnings = append(w.Earnings, Earning{
				AmountFen:   e.AmountFen.String(),
				LikesReward: e.LikesReward,
				EligibleAt:  e.EligibleAt,
				BatchID:     e.BatchID,
			})
		}

		out = append(out, w)
	}

	return out, nil
}

func (s *Service) Catalog(ctx context.Context, uid string) (Catalog, error) {
	policy, err := task.LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return Catalog{}, err
	}

	out := Catalog{
		Online: policy.CreativeOnline,
		Limit:  policy.CreativeLimit,
		Tasks:  []TaskDTO{},
	}
	out.Mine, err = s.Works(ctx, uid)
	if err != nil {
		return Catalog{}, err
	}

	for _, w := range out.Mine {
		if w.BusinessDate == s.Day() {
			out.Used++
		}
	}

	if out.Online {
		out.Tasks, err = s.Tasks(ctx, true)
		for i := range out.Tasks {
			out.Tasks[i].PublishFen = policy.PriceFen.String()
		}
	}

	return out, err
}
