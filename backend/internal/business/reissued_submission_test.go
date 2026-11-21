package business

import (
	"context"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
)

func TestReissueFreezesCurrentPriceAndOriginalPlatformCannotBeResubmitted(t *testing.T) {
	services := testService(t)
	ctx := context.Background()
	login, err := services.DemoLogin(ctx, "reissue-user")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := services.CreateTask(ctx, "admin", TaskInput{
		Title:     "重派保护",
		Body:      "发布作品",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := services.Claim(ctx, login.UserID, version.TaskID, "original-claim")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/original-work/",
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.ReviewAtRevision(ctx, "admin", assigned.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	newVersion, err := services.PublishVersion(ctx, "admin", version.TaskID, TaskInput{
		Title:     "新要求",
		Body:      "更新要求后重新派发",
		PriceFen:  "900",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	services.Now = func() time.Time {
		return time.Date(2026, 11, 17, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	}
	mode := enum.MarketingExpiryEndOfDay
	if _, err := services.UpdateMarketingSettings(ctx, "admin", task.MarketingSettingsInput{ExpiryMode: &mode}); err != nil {
		t.Fatal(err)
	}

	reissued, err := services.Reissue(ctx, "admin", assigned.ID, enum.PlatformDouyin, "作品已不可见", false)
	if err != nil {
		t.Fatal(err)
	}

	if reissued.PriceFen != "900" || reissued.VersionID != newVersion.ID {
		t.Fatal("reissue did not preserve dispatch-time version and price")
	}

	expectedDeadline := time.Date(2026, 11, 17, 16, 0, 0, 0, time.UTC)
	if reissued.ExpiresAt == nil || !reissued.ExpiresAt.Equal(expectedDeadline) {
		t.Fatalf("reissue should expire at Beijing midnight: %+v", reissued.ExpiresAt)
	}

	err = services.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/replaced-original/",
	})
	if err == nil {
		t.Fatal("invalid original platform was resurrected after reissue")
	}

	other, err := services.CreateTask(ctx, "admin", TaskInput{
		Title:     "其他任务",
		Body:      "另一个任务",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.Claim(ctx, login.UserID, other.TaskID, "another-daily-claim"); err == nil {
		t.Fatal("reissue did not consume the unused daily quota")
	}

	if err := services.Submit(ctx, login.UserID, reissued.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/reissued-work/",
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.ReviewAtRevision(ctx, "admin", reissued.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.Client.User.DeleteOneID(login.UserID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := services.Client.Task.DeleteOneID(version.TaskID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	claimRetry, err := services.Claim(ctx, login.UserID, version.TaskID, "original-claim")
	if err != nil || claimRetry.ID != assigned.ID {
		t.Fatalf("historical claim retry: %v, %v", claimRetry, err)
	}

	reissueRetry, err := services.Reissue(ctx, "admin", assigned.ID, enum.PlatformDouyin, "作品已不可见", false)
	if err != nil || reissueRetry.ID != reissued.ID {
		t.Fatalf("historical reissue retry: %v, %v", reissueRetry, err)
	}

	if _, err := services.Reissue(ctx, "admin", reissued.ID, enum.PlatformDouyin, "重新派发", false); err == nil {
		t.Error("new dispatch accepted archived user and task")
	}

	waived, err := services.Reissue(ctx, "admin", reissued.ID, enum.PlatformDouyin, "只保留原审核款项", true)
	if err != nil || waived.ID != reissued.ID {
		t.Fatalf("review of historical submission: %v, %v", waived, err)
	}
}
