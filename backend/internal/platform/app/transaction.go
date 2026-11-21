package app

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func Transaction[T any](ctx context.Context, runtime *Runtime, fn func(*ent.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := runtime.Client.Tx(ctx)
	if err != nil {
		return zero, err
	}

	defer tx.Rollback()
	result, err := fn(tx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return zero, Conflict("操作与已有记录冲突，请刷新后重试")
		}

		return zero, err
	}

	if err = tx.Commit(); err != nil {
		return zero, err
	}

	logging.From(ctx).Info().
		Msg("business transaction committed")

	return result, nil
}
