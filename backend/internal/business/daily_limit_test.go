package business

import (
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
)

func TestDailyLimitAppliesAcrossSelfAndManual(t *testing.T) {
	s := testService(t)
	ctx := t.Context()
	login, err := s.DemoLogin(ctx, "quota-1126")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	limit := 2
	if _, err := s.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{DailyLimit: &limit}); err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, title := range []string{"任务一", "任务二", "任务三"} {
		version, err := s.CreateTask(ctx, "operator", TaskInput{
			Title:     title,
			Body:      "发布",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin},
		})
		if err != nil {
			t.Fatal(err)
		}

		ids = append(ids, version.TaskID)
	}

	manual, err := s.Assign(ctx, "operator", login.UserID, ids[0], "manual-limit-1126")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimToday(ctx, login.UserID, "claim-limit-1126"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimToday(ctx, login.UserID, "third-limit-1126"); err == nil {
		t.Fatal("允许超过每日名额")
	}

	if err := s.Revoke(ctx, "operator", manual.ID, "释放一个名额"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimToday(ctx, login.UserID, "third-limit-1126"); err != nil {
		t.Fatal(err)
	}
}
