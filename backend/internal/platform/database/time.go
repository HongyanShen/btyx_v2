package database

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"
	"github.com/lib/pq"
)

// legacyTimeHook converts the old millisecond representation before Ent plans
// its schema diff. All conversions commit together; reruns only inspect bigint
// columns that the current Ent schema explicitly defines as times.
func legacyTimeHook(conn *sql.Conn, namespace string) schema.Hook {
	return func(next schema.Creator) schema.Creator {
		return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
			if err := migrateLegacyTimes(ctx, conn, namespace, tables); err != nil {
				return err
			}

			return next.Create(ctx, tables...)
		})
	}
}

func migrateLegacyTimes(ctx context.Context, conn *sql.Conn, namespace string, tables []*schema.Table) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT table_name, column_name FROM information_schema.columns WHERE table_schema = $1 AND data_type = 'bigint'`, namespace)
	if err != nil {
		return err
	}

	legacyColumns := make(map[string]bool)
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			rows.Close()

			return err
		}

		legacyColumns[tableName+"."+columnName] = true
	}

	if err := rows.Err(); err != nil {
		rows.Close()

		return err
	}

	if err := rows.Close(); err != nil {
		return err
	}

	for _, table := range tables {
		for _, column := range table.Columns {
			if column.Type != field.TypeTime || !legacyColumns[table.Name+"."+column.Name] {
				continue
			}

			name := pq.QuoteIdentifier(column.Name)
			source := name
			nullability := ""
			if column.Nullable {
				source = "NULLIF(" + name + ", 0)"
				nullability = ", ALTER COLUMN " + name + " DROP NOT NULL"
			}

			// Split seconds and milliseconds to preserve integer millisecond
			// precision, including pre-epoch values, without session-zone casts.
			statement := fmt.Sprintf(
				`ALTER TABLE %s.%s ALTER COLUMN %s DROP DEFAULT%s, ALTER COLUMN %s TYPE timestamptz USING (TIMESTAMPTZ 'epoch' + (%s / 1000) * INTERVAL '1 second' + (%s %% 1000) * INTERVAL '1 millisecond')`,
				pq.QuoteIdentifier(namespace),
				pq.QuoteIdentifier(table.Name),
				name,
				nullability,
				name,
				source,
				source,
			)
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("迁移时间字段 %s.%s: %w", table.Name, column.Name, err)
			}
		}
	}

	return tx.Commit()
}
