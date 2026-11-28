package business

import (
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
)

func TestConfiguredSettlementOpensPerPlatformAtExactTime(t *testing.T) {
	services := testService(t)
	ctx := t.Context()
	allocatedAt := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	now := allocatedAt
	services.Now = func() time.Time { return now }
	settings, err := services.DistributionSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}

	settings.ExpiryMinutes = 120
	settings.Schedule.SettlementFrequency = enum.SettlementAfterDelay
	settings.Schedule.SettlementDelayMinutes = 120
	settings.Schedule.PlaybackCutoffMinutes = 60
	settings.Schedule.PlaybackDeferralPeriods = 2
	if _, err := services.UpdateDistributionSettings(ctx, "operator", settings); err != nil {
		t.Fatal(err)
	}

	login, err := services.DemoLogin(ctx, "custom-settlement-schedule")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	platforms := []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu}
	version, err := services.CreateTask(ctx, "operator", TaskInput{
		Title:     "结算规则测试",
		Body:      "分发要求",
		PriceFen:  "750",
		Platforms: platforms,
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := services.Claim(ctx, login.UserID, version.TaskID, "claim-custom-schedule")
	if err != nil || assigned.ExpiresAt == nil || !assigned.ExpiresAt.Equal(allocatedAt.Add(2*time.Hour)) {
		t.Fatalf("assignment did not keep the configured link deadline: %+v, %v", assigned, err)
	}

	links := []string{"https://v.douyin.com/schedule-test/", "https://xhslink.cn/o/schedule-test"}
	for i, platform := range platforms {
		if err := services.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
			Platform: platform,
			URL:      links[i],
		}); err != nil {
			t.Fatal(err)
		}

		if err := services.ReviewAtRevision(ctx, "operator", assigned.ID, submission.ReviewInput{
			Platform: platform,
			Revision: 1,
			Approved: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	now = allocatedAt.Add(59 * time.Minute)
	if err := services.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform:   platforms[0],
		URL:        links[0],
		Screenshot: "on-time-screenshot",
	}); err != nil {
		t.Fatal(err)
	}

	now = allocatedAt.Add(time.Hour)
	if err := services.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform:   platforms[1],
		URL:        links[1],
		Screenshot: "late-screenshot",
	}); err != nil {
		t.Fatal(err)
	}

	settings.Schedule.SettlementFrequency = enum.SettlementMonthly
	if _, err := services.UpdateDistributionSettings(ctx, "operator", settings); err != nil {
		t.Fatal(err)
	}

	period := "2026-11-26"
	credit := settlement.SettlementInput{
		Through: period,
		Key:     "credit-custom-schedule",
	}
	now = allocatedAt.Add(2*time.Hour - time.Second)
	if _, err := services.Settle(ctx, "operator", credit); err == nil {
		t.Fatal("settled through direct API before opening time")
	}

	now = allocatedAt.Add(2 * time.Hour)
	report, err := services.Month(ctx, "2026-11", settlement.MonthAccess{Pending: true})
	if err != nil || len(report.Weeks) < 1 || report.Weeks[0].Through != period || report.Weeks[0].ReviewToken == "" {
		t.Fatalf("previously configured date is missing from review: %+v, %v", report, err)
	}

	credit.ReviewToken = report.Weeks[0].ReviewToken
	batch, err := services.Settle(ctx, "operator", credit)
	if err != nil || batch.Count != 1 || batch.AmountFen != "750" {
		t.Fatalf("first opening should credit only on-time platform: %+v, %v", batch, err)
	}

	now = allocatedAt.Add(5 * time.Hour)
	credit.Key = "credit-deferred-platform"
	credit.ReviewToken = ""
	batch, err = services.Settle(ctx, "operator", credit)
	if err != nil || batch.Count != 1 || batch.AmountFen != "750" {
		t.Fatalf("deferred platform should mature after two waiting periods: %+v, %v", batch, err)
	}
}
