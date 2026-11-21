package database_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestUserDomainMigrationPreservesLegacyAccount(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	ctx := context.Background()
	userID := app.NewID()
	nameCipher, err := services.Encrypt("旧用户")
	if err != nil {
		t.Fatal(err)
	}

	contactCipher, err := services.Encrypt("13800000001")
	if err != nil {
		t.Fatal(err)
	}

	verifiedCipher, err := services.Encrypt("13900000002")
	if err != nil {
		t.Fatal(err)
	}

	_, err = services.DB.ExecContext(ctx, `
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS openid text UNIQUE,
			ADD COLUMN IF NOT EXISTS name_cipher text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS identity_cipher text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS phone_cipher text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS verified_phone_cipher text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS douyin text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS xhs text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS enrollment_status text NOT NULL DEFAULT 'none',
			ADD COLUMN IF NOT EXISTS reason text NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS tags jsonb;
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = services.DB.ExecContext(ctx, `
		INSERT INTO users
			(id, created_at, updated_at, openid, name_cipher, identity_cipher, phone_cipher,
			 verified_phone_cipher, nickname, avatar_file_id, douyin, xhs, enrollment_status, reason, tags)
		VALUES ($1, now(), now(), 'local:legacy-member', $2, 'stored-identity-cipher', $3, $4,
		        '历史昵称', $5, '旧抖音', '旧小红书', 'approved', '历史备注', '["生活"]'::jsonb)
	`, userID, nameCipher, contactCipher, verifiedCipher, app.NewID())
	if err != nil {
		t.Fatal(err)
	}

	agreement, err := services.Client.Agreement.Create().
		SetTitle("历史协议").
		SetBody("历史协议内容").
		SetCreatorID("legacy-operator").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = services.Client.Consent.Create().
		SetUserID(userID).
		SetAgreementID(agreement.ID).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sessionToken := "legacy-session-test"
	err = services.Client.Session.Create().
		SetUserID(userID).
		SetTokenHash(app.Hash(sessionToken)).
		SetRole(enum.RoleUser).
		SetExpiresAt(time.Now().Add(time.Hour)).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, services.Config); err != nil {
			t.Fatal(err)
		}
	}

	var legacyColumns int
	err = services.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'users'
		  AND column_name IN ('openid', 'name_cipher', 'identity_cipher', 'phone_cipher',
		    'verified_phone_cipher', 'douyin', 'xhs', 'enrollment_status', 'reason', 'tags')
	`, services.Config.Schema).Scan(&legacyColumns)
	if err != nil || legacyColumns != 0 {
		t.Fatalf("legacy user columns remain after migration: count=%d, err=%v", legacyColumns, err)
	}

	login, err := services.DemoLogin(ctx, "legacy-member")
	if err != nil || login.UserID != userID {
		t.Fatalf("existing openid did not keep its user ID: %v", err)
	}

	principal, err := services.Authenticate(ctx, sessionToken)
	if err != nil || principal.ID != userID {
		t.Fatalf("existing session no longer resolves the migrated member: %v", err)
	}

	profile, err := services.User(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Name != "旧用户" || profile.Nickname != "历史昵称" || profile.AvatarFileID == "" ||
		profile.Phone != "138****0001" || profile.VerifiedPhone != "139****0002" ||
		profile.Douyin != "旧抖音" || profile.XHS != "旧小红书" ||
		profile.Status != enum.EnrollmentApproved || profile.Reason != "历史备注" ||
		!reflect.DeepEqual(profile.Tags, []string{"生活"}) || !profile.Consented {
		t.Fatal("migrated user profile fields or consent are incomplete")
	}
}
