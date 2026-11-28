package database_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/lib/pq"
)

func TestLegacyFullOperatorGrants(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	raw, err := os.ReadFile("legacy_operator_grants.json")
	if err != nil {
		t.Fatal(err)
	}

	var snapshot struct {
		Pages     []string
		Endpoints []string
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name      string
		pages     []string
		endpoints []string
	}{
		{"both", snapshot.Pages, snapshot.Endpoints},
		{"pages", snapshot.Pages, snapshot.Endpoints[1:]},
		{"endpoints", snapshot.Pages[1:], snapshot.Endpoints},
		{"partial", snapshot.Pages[1:], snapshot.Endpoints[1:]},
	} {
		err := service.Client.Operator.Create().
			SetUsername(test.name).
			SetPasswordHash("unused").
			SetPages(test.pages).
			SetEndpoints(test.endpoints).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	if _, err := service.DB.ExecContext(ctx, "ALTER TABLE operators DROP COLUMN all_pages, DROP COLUMN all_endpoints"); err != nil {
		t.Fatal(err)
	}

	for pass := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}

		accounts, err := service.Operators(ctx)
		if err != nil || len(accounts) != 4 {
			t.Fatalf("read migrated accounts: %v", err)
		}

		for _, account := range accounts {
			wantPages := account.Username == "pages" || (account.Username == "both" && pass == 0)
			wantEndpoints := account.Username == "both" || account.Username == "endpoints"
			if account.AllPages != wantPages || account.AllEndpoints != wantEndpoints {
				t.Fatalf("migration pass %d, %s: got pages=%t endpoints=%t", pass, account.Username, account.AllPages, account.AllEndpoints)
			}
		}

		if _, err := service.DB.ExecContext(ctx, "UPDATE operators SET all_pages=false WHERE username='both'"); err != nil {
			t.Fatal(err)
		}
	}
}

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
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v7;
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
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v7;
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
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v7;
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
		ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v7;
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
