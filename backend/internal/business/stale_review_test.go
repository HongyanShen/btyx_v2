package business

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
)

func TestReviewerCannotApproveAnUnseenResubmission(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	login, err := service.DemoLogin(ctx, "stale-review")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := service.CreateTask(ctx, "admin", TaskInput{

		Title:    "审核并发保护",
		Body:     "提交作品",
		PriceFen: "750",

		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := service.Claim(ctx, login.UserID, version.TaskID, "stale-claim")
	if err != nil {
		t.Fatal(err)
	}

	for _, link := range []string{"https://v.douyin.com/revision1/", "https://v.douyin.com/revision2/"} {
		if err := service.Submit(ctx, login.UserID, assigned.ID, SubmitInput{

			Platform: enum.PlatformDouyin,
			URL:      link,
		}); err != nil {
			t.Fatal(err)
		}
	}

	err = service.ReviewAtRevision(ctx, "admin", assigned.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	})
	if err == nil {
		t.Fatal("stale reviewer approved a newer submission")
	}
}
