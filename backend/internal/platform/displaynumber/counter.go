package displaynumber

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/tasknumbercounter"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/segmentio/ksuid"
)

type Counter string

const (
	Task     Counter = "task"
	Nickname Counter = "nickname"
)

func (counter Counter) Next(ctx context.Context, client *ent.Client) (int64, error) {
	query, args := entsql.Dialect(dialect.Postgres).
		Insert(tasknumbercounter.Table).
		Columns(tasknumbercounter.FieldID, tasknumbercounter.FieldName, tasknumbercounter.FieldCreatedAt, tasknumbercounter.FieldLastNumber).
		Values(ksuid.New().String(), string(counter), time.Now(), int64(1)).
		OnConflict(
			entsql.ConflictColumns(tasknumbercounter.FieldName),
			entsql.ResolveWith(func(update *entsql.UpdateSet) {
				update.Add(tasknumbercounter.FieldLastNumber, 1)
			}),
		).
		Returning(tasknumbercounter.FieldLastNumber).
		Query()

	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	defer rows.Close()
	return entsql.ScanInt64(rows)
}
