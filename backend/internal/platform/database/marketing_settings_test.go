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

func TestDistributionScheduleMigrationKeepsLegacySettings(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	minutes := 90
	if _, err := service.UpdateMarketingSettings(ctx, "admin", task.MarketingSettingsInput{ExpiryMinutes: &minutes}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DB.ExecContext(ctx, `ALTER TABLE marketing_settings
		DROP COLUMN settlement_frequency, DROP COLUMN settlement_weekday, DROP COLUMN settlement_month_day,
		DROP COLUMN settlement_clock, DROP COLUMN settlement_delay_minutes, DROP COLUMN playback_cutoff_clock,
		DROP COLUMN playback_cutoff_minutes, DROP COLUMN playback_deferral_periods`); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DB.ExecContext(ctx, `ALTER TABLE submissions DROP COLUMN settlement_eligible_at`); err != nil {
		t.Fatal(err)
	}

	for iteration := range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatalf("migration %d: %v", iteration, err)
		}
	}

	settings, err := service.DistributionSettings(ctx)
	if err != nil || settings.ExpiryMinutes != minutes || settings.Schedule.SettlementFrequency != enum.SettlementWeekly ||
		settings.Schedule.SettlementWeekday != 1 || settings.Schedule.SettlementClock != "00:00" ||
		settings.Schedule.PlaybackCutoffClock != "12:00" || settings.Schedule.PlaybackDeferralPeriods != 1 {
		t.Fatalf("legacy settings must retain the original behavior: %+v, %v", settings, err)
	}
}
