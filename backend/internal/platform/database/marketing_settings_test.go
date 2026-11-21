package database_test

import (
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestMarketingExpiryMigrationKeepsLegacyDuration(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	minutes := 90
	if _, err := service.UpdateMarketingSettings(ctx, "admin", task.MarketingSettingsInput{ExpiryMinutes: &minutes}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DB.ExecContext(ctx, `ALTER TABLE marketing_settings DROP COLUMN expiry_mode`); err != nil {
		t.Fatal(err)
	}

	for iteration := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	settings, err := service.MarketingSettings(ctx)
	if err != nil || settings.ExpiryMode != enum.MarketingExpiryFixedDuration || settings.ExpiryMinutes != minutes {
		t.Fatalf("migration lost legacy fixed settings: %+v, %v", settings, err)
	}

	if _, err := service.DB.ExecContext(ctx, `UPDATE marketing_settings SET expiry_mode = 'invalid'`); err == nil {
		t.Fatal("database accepted an invalid expiry mode")
	}
}
