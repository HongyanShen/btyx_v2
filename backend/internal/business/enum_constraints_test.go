package business

import (
	"context"
	"testing"
)

func TestDatabaseRejectsUnknownEnumsAndEnumCollections(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	login, err := service.DemoLogin(ctx, "enum-constraints")
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.DB.ExecContext(
		ctx,
		`UPDATE users SET enrollment_status = 'not_a_status' WHERE id = $1`,
		login.UserID,
	)
	if err == nil {
		t.Fatal("PostgreSQL accepted an invalid enum value")
	}

	_, err = service.DB.ExecContext(
		ctx,
		`INSERT INTO assignments
            (id, created_at, updated_at, user_id, task_id, version_id,
             price_fen, platforms, source, business_date, due_date,
             request_key, actor_id)
         VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $2, 'task', 'version', 750,
                 '["unknown_platform"]'::jsonb, 'claim',
                 '2026-11-16', '2026-11-16', $3, 'admin')`,
		NewID(),
		login.UserID,
		NewID(),
	)
	if err == nil {
		t.Fatal("PostgreSQL accepted an unknown platform in a collection")
	}
}
