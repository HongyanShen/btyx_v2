package database

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect/sql/schema"
	"github.com/lib/pq"
)

// Ent creates the domain tables first. The legacy columns are copied and
// removed in one PostgreSQL transaction so an interrupted copy can be retried.
func userDomainsHook(conn *sql.Conn, namespace string) schema.Hook {
	return func(next schema.Creator) schema.Creator {
		return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
			if err := next.Create(ctx, tables...); err != nil {
				return err
			}

			return splitUserDomains(ctx, conn, namespace)
		})
	}
}

func splitUserDomains(ctx context.Context, conn *sql.Conn, namespace string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var legacy, verifiedPhone bool
	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(bool_or(column_name = 'openid'), false),
			COALESCE(bool_or(column_name = 'verified_phone_cipher'), false)
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'users'
	`, namespace).Scan(&legacy, &verifiedPhone)
	if err != nil || !legacy {
		return err
	}

	quoted := pq.QuoteIdentifier(namespace)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`LOCK TABLE %s.users IN ACCESS EXCLUSIVE MODE`, quoted)); err != nil {
		return err
	}

	verifiedPhoneColumn := "''"
	if verifiedPhone {
		verifiedPhoneColumn = "verified_phone_cipher"
	}

	statements := []string{
		fmt.Sprintf(`
			INSERT INTO %[1]s.wechat_identities (id, user_id, openid, created_at)
			SELECT id, id, openid, created_at FROM %[1]s.users
		`, quoted),
		fmt.Sprintf(`
			INSERT INTO %[1]s.user_verifications
				(id, user_id, created_at, updated_at, name_cipher, identity_cipher, verified_phone_cipher)
			SELECT id, id, created_at, updated_at, name_cipher, identity_cipher, %[2]s
			FROM %[1]s.users
			WHERE name_cipher <> '' OR identity_cipher <> '' OR %[2]s <> ''
		`, quoted, verifiedPhoneColumn),
		fmt.Sprintf(`
			INSERT INTO %[1]s.user_platform_profiles
				(id, user_id, created_at, updated_at, douyin, xhs)
			SELECT id, id, created_at, updated_at, douyin, xhs FROM %[1]s.users
			WHERE douyin <> '' OR xhs <> ''
		`, quoted),
		fmt.Sprintf(`
			INSERT INTO %[1]s.user_enrollments
				(id, user_id, created_at, updated_at, status, contact_phone_cipher, reason, tags)
			SELECT id, id, created_at, updated_at, enrollment_status, phone_cipher, reason, tags
			FROM %[1]s.users
			WHERE enrollment_status <> 'none' OR phone_cipher <> '' OR reason <> '' OR tags IS NOT NULL
		`, quoted),
		fmt.Sprintf(`
			ALTER TABLE %s.users
				DROP COLUMN openid,
				DROP COLUMN name_cipher,
				DROP COLUMN identity_cipher,
				DROP COLUMN phone_cipher,
				DROP COLUMN IF EXISTS verified_phone_cipher,
				DROP COLUMN douyin,
				DROP COLUMN xhs,
				DROP COLUMN enrollment_status,
				DROP COLUMN reason,
				DROP COLUMN tags
		`, quoted),
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("迁移用户领域数据: %w", err)
		}
	}

	return tx.Commit()
}
