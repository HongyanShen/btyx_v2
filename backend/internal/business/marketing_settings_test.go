package business

import (
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
)

func TestMarketingSettingsApplyToNewAllocationsAndReleaseExpiredContent(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	originalTime := service.Now()
	currentTime := originalTime
	service.Now = func() time.Time { return currentTime }
	var members []string
	for _, device := range []string{"marketing-settings-a", "marketing-settings-b", "marketing-settings-c"} {
		login, err := service.DemoLogin(ctx, device)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
			t.Fatal(err)
		}

		members = append(members, login.UserID)
	}

	version, err := service.CreateTask(ctx, "operator", TaskInput{
		Title:     "待分配内容",
		Body:      "分发要求",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	firstPrice, firstDuration := "900", 30
	_, err = service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{
		PriceFen:      &firstPrice,
		ExpiryMinutes: &firstDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.ClaimToday(ctx, members[0], "claim-before-settings-change")
	if err != nil || first.PriceFen != "900" || first.ExpiresAt == nil || !first.ExpiresAt.Equal(originalTime.Add(30*time.Minute)) {
		t.Fatalf("first allocation did not freeze current settings: %+v, %v", first, err)
	}

	secondPrice, secondDuration := "1250", 120
	_, err = service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{
		PriceFen:      &secondPrice,
		ExpiryMinutes: &secondDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	settings, err := service.MarketingSettings(ctx)
	if err != nil || len(settings.PriceChanges) != 2 ||
		settings.PriceChanges[0].BeforeFen != "900" || settings.PriceChanges[0].AfterFen != "1250" ||
		settings.PriceChanges[1].BeforeFen != "750" || settings.PriceChanges[1].AfterFen != "900" {
		t.Fatalf("expected chronological price changes with both prices: %+v, %v", settings, err)
	}

	currentTime = originalTime.Add(30 * time.Minute)
	if err := service.Submit(ctx, members[0], first.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/expired-settings/",
	}); err == nil || err.Error() != "已超过发布链接提交时限" {
		t.Fatalf("late submissions must fail before the background scan: %v", err)
	}

	if err := service.ExpireMarketingAssignments(ctx); err != nil {
		t.Fatal(err)
	}

	frozen, err := service.Assignment(ctx, members[0], first.ID)
	if err != nil || frozen.Status != enum.AssignmentExpired || frozen.PriceFen != "900" || !frozen.ExpiresAt.Equal(*first.ExpiresAt) {
		t.Fatalf("old allocation settings changed or did not expire: %+v, %v", frozen, err)
	}

	pool, err := service.Tasks(ctx)
	if err != nil || len(pool) != 1 || pool[0].LastExpiry == nil ||
		pool[0].LastExpiry.UserID != members[0] || pool[0].Allocation != nil {
		t.Fatalf("released pool content lost its expiry history: %+v, %v", pool, err)
	}

	if _, err := service.ClaimToday(ctx, members[0], "claim-after-expiry"); err == nil {
		t.Fatal("expired assignment released the daily quota")
	}

	manual, err := service.Assign(ctx, "operator", members[1], version.TaskID, "assign-recycled-content")
	if err != nil || manual.PriceFen != "1250" || manual.ExpiresAt == nil || !manual.ExpiresAt.Equal(currentTime.Add(120*time.Minute)) {
		t.Fatalf("new assignee did not receive the recycled content at the new settings: %+v, %v", manual, err)
	}

	online := false
	_, err = service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{Online: &online})
	if err != nil {
		t.Fatal(err)
	}

	availability, err := service.TodayAvailability(ctx)
	if err != nil || availability.HasContent {
		t.Fatalf("offline campaign advertised content: %+v, %v", availability, err)
	}

	if _, err := service.ClaimToday(ctx, members[2], "claim-offline"); err == nil || err.Error() != "营销分发已下架" {
		t.Fatalf("campaign shutdown did not block new claims: %v", err)
	}

	if err := service.CompleteStep(ctx, members[1], manual.ID, enum.StepContent); err != nil {
		t.Fatal(err)
	}

	if err := service.Submit(ctx, members[1], manual.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/marketing-settings/",
	}); err != nil {
		t.Fatalf("existing assignment could not submit after campaign shutdown: %v", err)
	}

	currentTime = manual.ExpiresAt.Add(time.Minute)
	if err := service.ExpireMarketingAssignments(ctx); err != nil {
		t.Fatal(err)
	}

	submitted, err := service.Assignment(ctx, members[1], manual.ID)
	if err != nil || submitted.Status != enum.AssignmentPendingReview {
		t.Fatalf("submitted assignment expired: %+v, %v", submitted, err)
	}

	pool, err = service.Tasks(ctx)
	if err != nil || len(pool) != 1 || pool[0].Allocation == nil {
		t.Fatalf("expected the current allocation in the pool: %+v, %v", pool, err)
	}

	progress := pool[0].Allocation
	if len(progress.Steps) != 1 || progress.Steps[0] != enum.StepContent ||
		!progress.HasLink || !progress.LinksDone || progress.Reviewed || progress.Settled {
		t.Fatalf("pool progress lost explicit step and platform evidence: %+v", progress)
	}
}

func TestMarketingExpiryModesUseBeijingMidnightAndFreezeExistingDeadlines(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	allocatedAt := time.Date(2026, 11, 19, 14, 30, 0, 0, time.UTC)
	midnight := time.Date(2026, 11, 19, 16, 0, 0, 0, time.UTC)
	now := allocatedAt
	service.Now = func() time.Time { return now }
	members := make([]string, 0, 3)
	for _, device := range []string{"expiry-fixed", "expiry-day-assigned", "expiry-day-claimed"} {
		login, err := service.DemoLogin(ctx, device)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
			t.Fatal(err)
		}

		members = append(members, login.UserID)
	}

	var taskIDs []string
	for _, title := range []string{"固定时长内容", "当天手动指派", "当天自行领取"} {
		version, err := service.CreateTask(ctx, "operator", TaskInput{
			Title:     title,
			Body:      "发布要求",
			Platforms: []enum.Platform{enum.PlatformDouyin},
		})
		if err != nil {
			t.Fatal(err)
		}

		taskIDs = append(taskIDs, version.TaskID)
	}

	fixedMode, dayMode, duration := enum.MarketingExpiryFixedDuration, enum.MarketingExpiryEndOfDay, 120
	settings, err := service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{
		ExpiryMode:    &fixedMode,
		ExpiryMinutes: &duration,
	})
	if err != nil || settings.ExpiryMode != fixedMode {
		t.Fatalf("set fixed mode: %+v, %v", settings, err)
	}

	fixed, err := service.Claim(ctx, members[0], taskIDs[0], "expiry-fixed-claim")
	if err != nil || fixed.ExpiresAt == nil || !fixed.ExpiresAt.Equal(allocatedAt.Add(2*time.Hour)) {
		t.Fatalf("fixed allocation should cross midnight: %+v, %v", fixed, err)
	}

	settings, err = service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{ExpiryMode: &dayMode})
	if err != nil || settings.ExpiryMode != dayMode || settings.ExpiryMinutes != duration {
		t.Fatalf("day mode lost the saved fixed duration: %+v, %v", settings, err)
	}

	assigned, err := service.Assign(ctx, "operator", members[1], taskIDs[1], "expiry-day-assign")
	if err != nil || assigned.ExpiresAt == nil || !assigned.ExpiresAt.Equal(midnight) {
		t.Fatalf("manual assignment should expire at Beijing midnight: %+v, %v", assigned, err)
	}

	now = midnight.Add(-time.Minute)
	claimed, err := service.ClaimToday(ctx, members[2], "expiry-day-claim")
	if err != nil || claimed.ExpiresAt == nil || !claimed.ExpiresAt.Equal(midnight) || claimed.Date != "2026-11-19" {
		t.Fatalf("claim just before midnight used the wrong business date or deadline: %+v, %v", claimed, err)
	}

	settings, err = service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{ExpiryMode: &fixedMode})
	if err != nil || settings.ExpiryMode != fixedMode || settings.ExpiryMinutes != duration {
		t.Fatalf("switching back should restore the fixed duration: %+v, %v", settings, err)
	}

	if err := service.ExpireMarketingAssignments(ctx); err != nil {
		t.Fatal(err)
	}

	now = midnight
	if err := service.Submit(ctx, members[2], claimed.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/at-midnight/",
	}); err == nil || err.Error() != "已超过发布链接提交时限" {
		t.Fatalf("expected midnight submission to fail before worker runs: %v", err)
	}

	if err := service.ExpireMarketingAssignments(ctx); err != nil {
		t.Fatal(err)
	}

	retry, err := service.ClaimToday(ctx, members[2], "expiry-day-claim")
	if err != nil || retry.ID != claimed.ID || retry.Status != enum.AssignmentExpired {
		t.Fatalf("cross-midnight retry should return the original expired assignment: %+v, %v", retry, err)
	}

	stillFixed, err := service.Assignment(ctx, members[0], fixed.ID)
	if err != nil || stillFixed.Status != enum.AssignmentInProgress || stillFixed.ExpiresAt == nil || !stillFixed.ExpiresAt.Equal(*fixed.ExpiresAt) {
		t.Fatalf("already allocated fixed deadline changed: %+v, %v", stillFixed, err)
	}

	overLimit := 1470
	if _, err := service.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{ExpiryMinutes: &overLimit}); err == nil {
		t.Fatal("fixed duration above 24 hours was accepted")
	}
}
