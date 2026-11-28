package business

import (
	"context"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestSameWorkLinkCanBeSubmittedForDifferentAssignments(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	member, err := service.DemoLogin(ctx, "duplicate-work-link")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareTestUser(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	claim := func(title string) string {
		t.Helper()

		version, err := service.CreateTask(ctx, "admin", TaskInput{
			Title:     title,
			Body:      "发布作品",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin},
		})
		if err != nil {
			t.Fatal(err)
		}

		assigned, err := service.Claim(ctx, member.UserID, version.TaskID, title)
		if err != nil {
			t.Fatal(err)
		}

		return assigned.ID
	}

	submit := func(assignmentID, link string) error {
		return service.Submit(ctx, member.UserID, assignmentID, SubmitInput{
			Platform: enum.PlatformDouyin,
			URL:      link,
		})
	}

	first := claim("第一条任务")
	firstLink := "https://v.douyin.com/first-work/"
	if err := submit(first, firstLink); err != nil {
		t.Fatal(err)
	}

	firstDate := service.Now()
	service.Now = func() time.Time {
		return firstDate.AddDate(0, 0, 1)
	}

	second := claim("第二条任务")
	if err := submit(second, firstLink); err != nil {
		t.Fatalf("submitting the same link for another assignment: %v", err)
	}

	secondLink := "https://v.douyin.com/first-work?source=share#video"
	if err := submit(second, secondLink); err != nil {
		t.Fatalf("updating a pending submission with another share of the same work: %v", err)
	}

	if err := submit(second, secondLink); err != nil {
		t.Fatalf("retrying the second submission: %v", err)
	}

	current, err := service.Assignment(ctx, member.UserID, second)
	if err != nil {
		t.Fatal(err)
	}

	if len(current.Submissions) != 1 || current.Submissions[0].URL != secondLink ||
		current.Submissions[0].Revision != 2 {
		t.Fatalf("expected one updated submission after retry: %+v", current.Submissions)
	}
}
