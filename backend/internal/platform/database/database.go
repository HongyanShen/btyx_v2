package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	atlas "ariga.io/atlas/sql/schema"
	"baituber.cn/yanxuan/v2/ent"
	_ "baituber.cn/yanxuan/v2/ent/runtime"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"
)

func Open(ctx context.Context, c config.Config) (*sql.DB, *ent.Client, error) {
	db, err := sql.Open("postgres", c.DSN(true))
	if err != nil {
		return nil, nil, err
	}

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("连接本机 PostgreSQL 失败: %w", err)
	}

	return db, ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db))), nil
}

func Migrate(ctx context.Context, c config.Config) error {
	if !config.SchemaName.MatchString(c.Schema) {
		return fmt.Errorf("invalid schema")
	}

	db, err := sql.Open("postgres", c.DSN(false))
	if err != nil {
		return err
	}

	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, "btyx-migrate:"+c.Schema); err != nil {
		return err
	}

	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, "btyx-migrate:"+c.Schema)
	if _, err = conn.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS "`+c.Schema+`"`); err != nil {
		return err
	}

	legacySoftDeleteTables, err := prepareLegacySoftDeletes(ctx, conn, c.Schema)
	if err != nil {
		return err
	}

	_, client, err := Open(ctx, c)
	if err != nil {
		return err
	}

	defer client.Close()
	if err = client.Schema.Create(
		ctx,
		schema.WithDropIndex(true),
		schema.WithDiffHook(keepUnrelatedIndexes),
		schema.WithHooks(legacyTimeHook(conn, c.Schema), userDomainsHook(conn, c.Schema)),
	); err != nil {
		return err
	}

	if err = finishLegacySoftDeletes(ctx, conn, c.Schema, legacySoftDeleteTables); err != nil {
		return err
	}

	// Enforce append-only records at the database boundary as well as in services.
	for _, table := range []string{
		"ledger_entries",
		"audit_events",
		"task_versions",
		"settlement_lines",
		"settlement_batches",
		"consents",
		"marketing_tag_deletions",
	} {
		statement := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %[1]s.reject_record_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'immutable record: %%', TG_TABLE_NAME;
END $$;

DROP TRIGGER IF EXISTS immutable_record ON %[1]s.%[2]s;

CREATE TRIGGER immutable_record
BEFORE UPDATE OR DELETE ON %[1]s.%[2]s
FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_record_change();
`, c.Schema, table)
		if _, err = conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	walletConstraint := fmt.Sprintf(`
ALTER TABLE %[1]s.wallets DROP CONSTRAINT IF EXISTS wallet_nonnegative;

ALTER TABLE %[1]s.wallets ADD CONSTRAINT wallet_nonnegative
CHECK (
    available_fen >= 0
    AND frozen_fen >= 0
    AND withdrawn_fen >= 0
);
`, c.Schema)

	_, err = conn.ExecContext(ctx, walletConstraint)
	return err
}

func keepUnrelatedIndexes(next schema.Differ) schema.Differ {
	return schema.DiffFunc(func(current, desired *atlas.Schema) ([]atlas.Change, error) {
		changes, err := next.Diff(current, desired)
		if err != nil {
			return nil, err
		}

		for _, change := range changes {
			table, ok := change.(*atlas.ModifyTable)
			if !ok {
				continue
			}

			allowed := table.Changes[:0]
			for _, change := range table.Changes {
				drop, dropping := change.(*atlas.DropIndex)
				if !dropping || replacedIndex(table.T.Name, drop.I.Name) {
					allowed = append(allowed, change)
				}
			}

			table.Changes = allowed
		}

		return changes, nil
	})
}

func replacedIndex(table, name string) bool {
	return table == "user_enrollments" && name == "user_enrollments_user_id_key" ||
		table == "marketing_tags" && (name == "marketing_tags_name_key" || name == "marketing_tags_key_key")
}

func DropTest(ctx context.Context, c config.Config) error {
	if !strings.HasPrefix(c.Schema, "btyx_test_") || !config.SchemaName.MatchString(c.Schema) {
		return fmt.Errorf("拒绝清理非测试 schema")
	}

	db, err := sql.Open("postgres", c.DSN(false))
	if err != nil {
		return err
	}

	defer db.Close()
	_, err = db.ExecContext(ctx, `DROP SCHEMA "`+c.Schema+`" CASCADE`)
	return err
}
