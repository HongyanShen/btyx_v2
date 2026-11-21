package settlement

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/settlementbatch"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func existingBatch(ctx context.Context, tx *ent.Tx, input SettlementInput) (*ent.SettlementBatch, error) {
	batch, err := tx.SettlementBatch.Query().
		Where(settlementbatch.RequestKeyEQ(input.Key)).
		Only(ctx)
	if err != nil {
		return nil, err
	}

	if batch.Through != input.Through {
		return nil, app.Conflict("幂等键与原结算请求不一致")
	}

	return batch, nil
}

func batchDTO(batch *ent.SettlementBatch) BatchDTO {
	return BatchDTO{
		ID:        batch.ID,
		Through:   batch.Through,
		AmountFen: batch.AmountFen.String(),
		Count:     batch.Count,
		CreatorID: batch.CreatorID,
	}
}

func (s *Service) Batches(ctx context.Context) ([]BatchDTO, error) {
	rows, err := s.Client.SettlementBatch.Query().
		Order(ent.Desc(settlementbatch.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := []BatchDTO{}
	for _, batch := range rows {
		out = append(out, batchDTO(batch))
	}

	return out, nil
}
