package business

import (
	"slices"
	"sync"
	"testing"

	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestPoolShowsActualMediaAndOriginalUploadTime(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	video, err := service.Client.FileObject.Create().
		SetOwnerID("operator").
		SetName("demo.mp4").
		SetMediaType(enum.FileMP4).
		SetSize(10).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	input := TaskInput{
		Title:     "带视频的营销内容",
		Body:      "按要求发布",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
		Files:     []string{video.ID},
	}
	first, err := service.CreateTask(ctx, "operator", input)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.PublishVersion(ctx, "operator", first.TaskID, input)
	if err != nil {
		t.Fatal(err)
	}

	pool, err := service.Tasks(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(pool) != 1 || pool[0].ID != updated.ID || !slices.Equal(pool[0].MediaTypes, []enum.FileMediaType{enum.FileMP4}) || pool[0].UploadedAt == nil || pool[0].UploadedAt.After(first.CreatedAt) {
		t.Fatalf("expected original upload time and video type for the current version, got %+v", pool)
	}
}

func TestClaimTodaySelectsAvailableContentAndSharesManualQuota(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	userIDs := make([]string, 0, 3)
	for _, device := range []string{"today-first", "today-second", "today-third"} {
		login, err := service.DemoLogin(ctx, device)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
			t.Fatal(err)
		}

		userIDs = append(userIDs, login.UserID)
	}

	tag, err := service.CreateMarketingTag(ctx, "operator", "影视")
	if err != nil {
		t.Fatal(err)
	}

	for _, userID := range userIDs {
		err := service.Client.UserEnrollment.Update().
			Where(userenrollment.UserIDEQ(userID), userenrollment.IsCurrentEQ(true)).
			SetTags([]string{tag.Key}).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	create := func(title string, tags []string, deadline string) VersionDTO {
		t.Helper()
		version, err := service.CreateTask(ctx, "operator", TaskInput{
			Title:     title,
			Body:      "按要求发布",
			Project:   "测试艺人",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin},
			Tags:      tags,
			Deadline:  deadline,
		})
		if err != nil {
			t.Fatal(err)
		}

		material, err := service.Client.Content.Get(ctx, version.ContentID)
		if err != nil || material.Title != title || material.Project != "测试艺人" {
			t.Fatalf("created task missing from content pool: %+v, %v", material, err)
		}

		return version
	}

	preferred := create("优先匹配", []string{tag.Key}, "")
	fallback := create("无标签也可领取", nil, "")
	create("晚入池的无标签内容", nil, "")
	manual := create("手动指派", []string{tag.Key}, "")
	create("已截止", []string{tag.Key}, "2000-01-01")
	offline := create("已下架", []string{tag.Key}, "")
	if err := service.SetOnline(ctx, "operator", offline.TaskID, false); err != nil {
		t.Fatal(err)
	}

	availability, err := service.TodayAvailability(ctx)
	if err != nil || !availability.HasContent {
		t.Fatalf("expected claimable content, got %+v, %v", availability, err)
	}

	first, err := service.ClaimToday(ctx, userIDs[0], "today-first-key")
	if err != nil || first.TaskID != manual.TaskID || first.VersionID != manual.ID || first.PriceFen != "750" {
		t.Fatalf("expected latest matching task and its snapshot, got %+v, %v", first, err)
	}

	repeated, err := service.ClaimToday(ctx, userIDs[0], "today-first-key")
	if err != nil || repeated.ID != first.ID {
		t.Fatalf("claim retry returned %+v, %v, want %s", repeated, err, first.ID)
	}

	if _, err := service.Assign(ctx, "operator", userIDs[0], preferred.TaskID, "manual-after-claim"); err == nil {
		t.Fatal("manual assignment ignored the claim quota")
	}

	assigned, err := service.Assign(ctx, "operator", userIDs[1], preferred.TaskID, "manual-before-claim")
	if err != nil {
		t.Fatal(err)
	}

	pool, err := service.Tasks(ctx)
	if err != nil {
		t.Fatal(err)
	}

	allocations := make(map[string]VersionDTO, len(pool))
	for _, row := range pool {
		allocations[row.TaskID] = row
	}

	manualAllocation := allocations[preferred.TaskID].Allocation
	if manualAllocation == nil || manualAllocation.ID != assigned.ID || manualAllocation.Source != enum.SourceAssigned || manualAllocation.HasLink {
		t.Fatalf("manual assignment not reflected in the pool: %+v", manualAllocation)
	}

	if _, err := service.ClaimToday(ctx, userIDs[1], "claim-after-manual"); err == nil {
		t.Fatal("claim ignored the manual assignment quota")
	}

	last, err := service.ClaimToday(ctx, userIDs[2], "today-fallback-key")
	if err != nil || last.TaskID != fallback.TaskID {
		t.Fatalf("expected oldest fallback task, got %+v, %v", last, err)
	}

	if err := service.Revoke(ctx, "operator", assigned.ID, "指派错误"); err != nil {
		t.Fatal(err)
	}

	afterRevoke, err := service.ClaimToday(ctx, userIDs[1], "after-revoke")
	if err != nil || afterRevoke.TaskID != preferred.TaskID {
		t.Fatalf("withdrawn assignment must release both content and quota: %+v, %v", afterRevoke, err)
	}
}

func TestConcurrentDailyClaimsPickDifferentContent(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	var userIDs []string
	for _, title := range []string{"并发任务甲", "并发任务乙"} {
		login, err := service.DemoLogin(ctx, title)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
			t.Fatal(err)
		}

		userIDs = append(userIDs, login.UserID)
		if _, err := service.CreateTask(ctx, "operator", TaskInput{
			Title:     title,
			Body:      "按要求发布",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin},
		}); err != nil {
			t.Fatal(err)
		}
	}

	var waiting sync.WaitGroup
	results := make(chan AssignmentDTO, len(userIDs))
	for _, userID := range userIDs {
		waiting.Go(func() {
			assigned, err := service.ClaimToday(ctx, userID, NewID())
			if err != nil {
				t.Error(err)
			}

			results <- assigned
		})
	}

	waiting.Wait()
	close(results)
	seen := make(map[string]bool, len(userIDs))
	for assigned := range results {
		if assigned.TaskID == "" || seen[assigned.TaskID] {
			t.Fatalf("content was missing or allocated twice: %+v", assigned)
		}

		seen[assigned.TaskID] = true
	}

	availability, err := service.TodayAvailability(ctx)
	if err != nil || availability.HasContent {
		t.Fatalf("allocated tasks must disappear from availability: %+v, %v", availability, err)
	}
}
