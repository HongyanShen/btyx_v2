package database_test

import (
	"context"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/lib/pq"
)

func TestOperatorMigrationPreservesExistingAudits(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	legacyActions := []string{}
	for _, action := range enum.AuditAction("").Values() {
		if action != string(enum.AuditOperatorCreate) && action != string(enum.AuditOperatorUpdate) &&
			action != string(enum.AuditUserProfileUpdate) && action != string(enum.AuditUserPhoneVerify) &&
			action != string(enum.AuditUserIdentityVerify) && action != string(enum.AuditUserIdentityFailed) &&
			action != string(enum.AuditEnrollmentIdentityVerify) && action != string(enum.AuditEnrollmentIdentityFailed) {
			legacyActions = append(legacyActions, pq.QuoteLiteral(action))
		}
	}

	_, err := service.DB.ExecContext(ctx, `
		DROP TABLE operators;
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v6;
		ALTER TABLE audit_events ADD CONSTRAINT action_enum CHECK (action IN (`+strings.Join(legacyActions, ", ")+`));
	`)
	if err != nil {
		t.Fatal(err)
	}

	existing, err := service.Client.AuditEvent.Create().
		SetActorID("admin").
		SetAction(enum.AuditTaskCreate).
		SetReferenceID("legacy-task").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for iteration := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	if err := service.BootstrapAdmin(ctx, "migration-admin", "test-migration-password"); err != nil {
		t.Fatal(err)
	}

	preserved, err := service.Client.AuditEvent.Get(ctx, existing.ID)
	if err != nil || preserved.ActorID != "admin" || preserved.Action != enum.AuditTaskCreate {
		t.Fatalf("legacy audit changed: %v, %v", preserved, err)
	}

	hasNewAudit, err := service.Client.AuditEvent.Query().
		Where(auditevent.ActionEQ(enum.AuditOperatorCreate)).
		Exist(ctx)
	if err != nil || !hasNewAudit {
		t.Fatalf("migration did not accept operator audit actions: %v", err)
	}
}

func TestIdentityHistoryMigrationReplacesPreviousAuditCheck(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()

	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v6;
		ALTER TABLE audit_events ADD CONSTRAINT audit_action_enum_v3
			CHECK (action NOT IN (
				'user.identity_verify_failed', 'enrollment.identity_verify', 'enrollment.identity_verify_failed'
			));
	`)
	if err != nil {
		t.Fatal(err)
	}

	previous, err := service.Client.AuditEvent.Create().
		SetActorID("member").
		SetReferenceID("member").
		SetAction(enum.AuditUserIdentityVerify).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(ctx, service.Config); err != nil {
		t.Fatal(err)
	}

	err = service.Client.AuditEvent.Create().
		SetActorID("member").
		SetReferenceID("member").
		SetAction(enum.AuditUserIdentityFailed).
		SetReason("姓名和身份证号不一致").
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	preserved, err := service.Client.AuditEvent.Get(ctx, previous.ID)
	if err != nil || preserved.Action != enum.AuditUserIdentityVerify {
		t.Fatal("successful identity history was not retained")
	}
}

func TestMarketingReviewMigrationReplacesExistingAuditCheck(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()

	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v6;
		ALTER TABLE audit_events ADD CONSTRAINT audit_action_enum_v4
			CHECK (action NOT IN ('enrollment.portrait_updated', 'marketing_tag.create', 'marketing_tag.rename'));
	`)
	if err != nil {
		t.Fatal(err)
	}

	for iteration := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	tag, err := service.CreateMarketingTag(ctx, "admin", "运动")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.RenameMarketingTag(ctx, "admin", tag.ID, "健身", tag.Name); err != nil {
		t.Fatal(err)
	}

	err = service.Client.AuditEvent.Create().
		SetActorID("admin").
		SetReferenceID("member").
		SetAction(enum.AuditEnrollmentPortrait).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMarketingSettingsMigrationReplacesPreviousAuditCheck(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	legacyActions := []string{}
	for _, action := range enum.AuditAction("").Values() {
		if action != string(enum.AuditMarketingSettings) && action != string(enum.AuditTaskExpire) {
			legacyActions = append(legacyActions, pq.QuoteLiteral(action))
		}
	}

	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v6;
		ALTER TABLE audit_events ADD CONSTRAINT audit_action_enum_v5
			CHECK (action IN (`+strings.Join(legacyActions, ", ")+`));
	`)
	if err != nil {
		t.Fatal(err)
	}

	previous, err := service.Client.AuditEvent.Create().
		SetActorID("admin").
		SetReferenceID("legacy-task").
		SetAction(enum.AuditTaskCreate).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for iteration := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	for _, expiry := range []int{90, 120} {
		settings, err := service.UpdateMarketingSettings(ctx, "admin", task.MarketingSettingsInput{
			ExpiryMinutes: &expiry,
		})
		if err != nil || settings.ExpiryMinutes != expiry {
			t.Fatalf("save %d minutes after upgrading legacy database: %+v, %v", expiry, settings, err)
		}

		settings, err = service.MarketingSettings(ctx)
		if err != nil || settings.ExpiryMinutes != expiry {
			t.Fatalf("read %d minutes after save: %+v, %v", expiry, settings, err)
		}
	}

	err = service.Client.AuditEvent.Create().
		SetActorID("worker").
		SetReferenceID("assignment").
		SetAction(enum.AuditTaskExpire).
		Exec(ctx)
	if err != nil {
		t.Fatalf("audit check rejected the expiry event after migration: %v", err)
	}

	preserved, err := service.Client.AuditEvent.Get(ctx, previous.ID)
	if err != nil || preserved.Action != enum.AuditTaskCreate {
		t.Fatalf("legacy audit changed: %v, %v", preserved, err)
	}
}
