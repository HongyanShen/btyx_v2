package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// Populate the flag before Ent creates the partial unique indexes. Archived
// tags can share a name with an active tag in the previous schema.
func prepareLegacySoftDeletes(ctx context.Context, conn *sql.Conn, namespace string) ([]string, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()
	var legacyTables []string
	for _, table := range []string{"contents", "marketing_tags", "tasks", "users"} {
		var hasLegacyColumn bool
		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_schema = $1 AND table_name = $2 AND column_name = 'deleted_at' AND data_type = 'bigint'
			)
		`, namespace, table).Scan(&hasLegacyColumn)
		if err != nil {
			return nil, err
		}

		if !hasLegacyColumn {
			continue
		}

		qualifiedTable := pq.QuoteIdentifier(namespace) + "." + pq.QuoteIdentifier(table)
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+qualifiedTable+` ADD COLUMN IF NOT EXISTS deleted boolean NOT NULL DEFAULT false`); err != nil {
			return nil, fmt.Errorf("add soft delete flag to %s: %w", table, err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE `+qualifiedTable+` SET deleted = true WHERE deleted_at > 0 AND NOT deleted`); err != nil {
			return nil, fmt.Errorf("backfill soft delete flag on %s: %w", table, err)
		}

		legacyTables = append(legacyTables, table)
	}

	return legacyTables, tx.Commit()
}

// Keep the original tag deletion timestamp in the immutable deletion record.
// Ent applies the new indexes and creates the deletion table before this runs.
func finishLegacySoftDeletes(ctx context.Context, conn *sql.Conn, namespace string, tables []string) error {
	if len(tables) == 0 {
		return nil
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	quotedNamespace := pq.QuoteIdentifier(namespace)
	for _, table := range tables {
		if table == "marketing_tags" {
			statement := fmt.Sprintf(`
INSERT INTO %[1]s.marketing_tag_deletions (id, created_at, key)
SELECT id, TIMESTAMPTZ 'epoch' + (deleted_at / 1000) * INTERVAL '1 second'
                           + (deleted_at %% 1000) * INTERVAL '1 millisecond', key
FROM %[1]s.marketing_tags
WHERE deleted_at > 0
ON CONFLICT (id) DO NOTHING`, quotedNamespace)
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("backfill retired tag keys: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+quotedNamespace+`.`+pq.QuoteIdentifier(table)+` DROP COLUMN deleted_at`); err != nil {
			return fmt.Errorf("drop legacy soft delete field on %s: %w", table, err)
		}
	}

	return tx.Commit()
}
