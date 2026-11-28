package business

import (
	"context"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent/tasknumbercounter"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/displaynumber"
)

func TestRegistrationAndTasksUseIndependentNumbers(t *testing.T) {
	service := testService(t)
	ctx := t.Context()
	first, err := service.DemoLogin(ctx, "numbered-user")
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.Client.User.Get(ctx, first.UserID)
	if err != nil || member.Nickname != "微信用户UkLW" {
		t.Fatalf("first auto nickname: %v, %v", member, err)
	}

	version, err := service.CreateTask(ctx, "admin", TaskInput{
		Title:     "独立编号",
		Body:      "发布要求",
		Platforms: []enum.Platform{enum.PlatformDouyin},
		PriceFen:  "750",
	})
	if err != nil || version.TaskCode != "RW-UkLW" {
		t.Fatalf("first task number after registration: %q, %v", version.TaskCode, err)
	}

	if err := service.Client.User.UpdateOneID(first.UserID).SetNickname("已修改的昵称").Exec(ctx); err != nil {
		t.Fatal(err)
	}

	again, err := service.DemoLogin(ctx, "numbered-user")
	if err != nil || again.UserID != first.UserID {
		t.Fatalf("repeat login: %v", err)
	}

	member, err = service.Client.User.Get(ctx, first.UserID)
	if err != nil || member.Nickname != "已修改的昵称" {
		t.Fatalf("repeat login changed nickname: %v, %v", member, err)
	}

	if _, err := service.Client.User.Create().SetNickname("微信用户gbHJ").Save(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err := service.Client.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer tx.Rollback()
	_, err = tx.TaskNumberCounter.Query().
		Where(tasknumbercounter.NameEQ(string(displaynumber.Task))).
		ForUpdate().
		Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A registration must complete while another transaction holds the task counter row.
	loginCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	second, err := service.DemoLogin(loginCtx, "another-numbered-user")
	if err != nil {
		t.Fatal(err)
	}

	member, err = service.Client.User.Get(ctx, second.UserID)
	if err != nil || member.Nickname != "微信用户Efhx" {
		t.Fatalf("new auto nickname should skip occupied name: %v, %v", member, err)
	}
}
