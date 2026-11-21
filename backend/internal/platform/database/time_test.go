package database_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent/migrate"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"entgo.io/ent/schema/field"
	"github.com/lib/pq"
	"github.com/segmentio/ksuid"
)

func TestTimeMigrationPreservesLegacyMilliseconds(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.Load("../../../../.env")
	if err != nil {
		t.Fatal(err)
	}

	cfg.Schema = "btyx_test_" + strings.ToLower(ksuid.New().String())
	t.Cleanup(func() {
		if err := database.DropTest(context.Background(), cfg); err != nil {
			t.Error(err)
		}
	})
	if err := database.Migrate(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("postgres", cfg.DSN(true))
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	var columnType string
	if err := db.QueryRowContext(ctx, `SELECT data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = 'sessions' AND column_name = 'expires_at'`, cfg.Schema).Scan(&columnType); err != nil {
		t.Fatal(err)
	}

	if columnType != "timestamp with time zone" {
		t.Fatalf("session expiry must be timestamptz, got %s", columnType)
	}

	// Recreate the old representation in this test's isolated schema, including
	// zero sentinels and defaults. Existing immutable-record triggers stay enabled.
	for _, table := range migrate.Tables {
		for _, column := range table.Columns {
			if column.Type != field.TypeTime {
				continue
			}

			name := pq.QuoteIdentifier(column.Name)
			statement := "ALTER TABLE " + pq.QuoteIdentifier(table.Name) + " ALTER COLUMN " + name + " DROP DEFAULT, ALTER COLUMN " + name + " TYPE bigint USING COALESCE((EXTRACT(EPOCH FROM " + name + ") * 1000)::bigint, 0)"
			if column.Nullable {
				statement += ", ALTER COLUMN " + name + " SET NOT NULL, ALTER COLUMN " + name + " SET DEFAULT 0"
			}

			if _, err := db.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
		}
	}

	const instant = "2026-11-17T10:30:45.123+08:00"
	want, err := time.Parse(time.RFC3339Nano, instant)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (id, created_at, token_hash, user_id, role, expires_at) VALUES ('session', $1, 'hash', 'user', 'user', $2)`, want.UnixMilli(), want.Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO file_objects (id, created_at, owner_id, name, media_type, size) VALUES ('file', $1, 'user', 'test.png', 'image/png', 8)`, want.UnixMilli()); err != nil {
		t.Fatal(err)
	}

	for iteration := 0; iteration < 2; iteration++ {
		if err := database.Migrate(ctx, cfg); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	var createdAt, expiresAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT created_at, expires_at FROM sessions WHERE id = 'session'`).Scan(&createdAt, &expiresAt); err != nil {
		t.Fatal(err)
	}

	if !createdAt.Equal(want) || !expiresAt.Equal(want.Add(time.Hour)) {
		t.Fatalf("migration changed instants: created=%s expiry=%s", createdAt, expiresAt)
	}

	var uploadExpiresAt, verifiedAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT upload_expires_at, verified_at FROM file_objects WHERE id = 'file'`).Scan(&uploadExpiresAt, &verifiedAt); err != nil {
		t.Fatal(err)
	}

	if uploadExpiresAt.Valid || verifiedAt.Valid {
		t.Fatal("absent legacy times must become NULL")
	}

	for _, table := range migrate.Tables {
		for _, column := range table.Columns {
			if column.Type != field.TypeTime {
				continue
			}

			if err := db.QueryRowContext(ctx, `SELECT data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`, cfg.Schema, table.Name, column.Name).Scan(&columnType); err != nil {
				t.Fatal(err)
			}

			const wantType = "timestamp with time zone"
			if columnType != wantType {
				t.Errorf("%s.%s = %s, want %s", table.Name, column.Name, columnType, wantType)
			}
		}
	}
}
