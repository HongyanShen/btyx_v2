package marketingtags

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/marketingtagdeletion"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

// Lock serializes library mutations with updates to tags stored in JSON arrays.
func Lock(ctx context.Context, tx *ent.Tx) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':marketing-tags', 0))`)
	return err
}

func CheckBindings(ctx context.Context, tx *ent.Tx, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := Lock(ctx, tx); err != nil {
		return err
	}

	removed, err := tx.MarketingTagDeletion.Query().
		Where(marketingtagdeletion.KeyIn(keys...)).
		Exist(ctx)
	if err != nil {
		return err
	}

	if removed {
		return app.Conflict("所选标签已删除，请刷新标签库后重新选择")
	}

	return nil
}
