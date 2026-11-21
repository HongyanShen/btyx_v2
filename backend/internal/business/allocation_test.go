package business

import (
	"context"
	"sync"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestConcurrentUsersCannotClaimTheSameMarketingContent(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	users := make([]string, 0, 2)

	for _, device := range []string{"allocation-first", "allocation-second"} {
		login, err := service.DemoLogin(ctx, device)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
			t.Fatal(err)
		}

		users = append(users, login.UserID)
	}

	version, err := service.CreateTask(ctx, "admin", TaskInput{
		Title:     "独占分发内容",
		Body:      "同一条内容只能分配给一名分发员",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	var waiting sync.WaitGroup
	results := make(chan error, len(users))
	for _, userID := range users {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			_, err := service.Claim(ctx, userID, version.TaskID, NewID())
			results <- err
		}()
	}

	waiting.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}

	if succeeded != 1 {
		t.Fatalf("same content allocated %d times", succeeded)
	}
}
