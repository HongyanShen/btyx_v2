package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	entsubmission "baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

func TestAdminBrowser(t *testing.T) {
	if os.Getenv("BTYX_ADMIN_BROWSER") != "1" {
		t.Skip("设置 BTYX_ADMIN_BROWSER=1 运行本地 Edge 和 PostgreSQL 浏览器回归")
	}

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}

	services := testkit.Service(t, root)
	services.Now = func() time.Time {
		return time.Date(2026, 11, 16, 10, 0, 0, 0, beijing)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const username = "browser-admin"
	const password = "browser-check-only-2026"
	if err := services.BootstrapAdmin(ctx, username, password); err != nil {
		t.Fatal(err)
	}

	account, err := services.LoginAdmin(ctx, username, password)
	if err != nil {
		t.Fatal(err)
	}

	if err := services.PublishAgreement(ctx, account.UserID, "浏览器验收协议", "本协议仅用于浏览器测试"); err != nil {
		t.Fatal(err)
	}

	reviewer, err := services.CreateOperator(ctx, account.UserID, identity.CreateOperatorInput{
		Username: "browser-reviewer",
		Password: password,
		OperatorGrants: identity.OperatorGrants{
			Pages:     []string{"/feedback", "/audits", "/tasks"},
			Endpoints: []string{"GET /feedback", "GET /tasks", "GET /marketing-settings"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	review := browserAssignment(t, ctx, services, account.UserID, "双平台待审核任务", []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu})
	approval := browserAssignment(t, ctx, services, account.UserID, "双平台通过验收任务", []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu})
	credit := browserAssignment(t, ctx, services, account.UserID, "结算验收任务", []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu})
	if err := services.ReviewAtRevision(ctx, account.UserID, credit.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.ReviewAtRevision(ctx, account.UserID, credit.ID, submission.ReviewInput{
		Platform: enum.PlatformXiaohongshu,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	browserSettlementFixtures(t, ctx, services, credit)

	if err := services.CreateFeedback(ctx, review.UserID, "浏览器验收反馈"); err != nil {
		t.Fatal(err)
	}

	file, err := services.Client.FileObject.Create().
		SetOwnerID(account.UserID).
		SetName("browser-upload.png").
		SetBucket("browser-check").
		SetObjectKey(business.NewID()).
		SetStorage(enum.FileStorageTOS).
		SetStatus(enum.FileReady).
		SetMediaType(enum.FilePNG).
		SetSize(8).
		SetVerifiedAt(services.Now()).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	member, err := services.DemoLogin(ctx, "htmx-assign-revoke")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetApplicantNickname("指派验收分发员").
		SetStatus(enum.EnrollmentApproved).
		SetRecentPost(true).
		SetNewAccount(false).
		SetAgeGroup(enum.EnrollmentAge25To30).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := services.Consent(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	nickname := "指派验收分发员"
	if err := services.UpdateProfile(ctx, member.UserID, identity.UpdateProfileInput{Nickname: &nickname}); err != nil {
		t.Fatal(err)
	}

	browserIdentityFixtures(t, ctx, services)
	browserEnrollmentFixture(t, ctx, services)

	services.Now = func() time.Time {
		return time.Date(2026, 11, 23, 14, 0, 0, 0, beijing)
	}

	_, err = services.Settle(ctx, account.UserID, settlement.SettlementInput{
		Through:     "2026-11-23",
		ReviewToken: "stale-preview",
		Key:         business.NewID(),
	})
	var conflict *app.Error
	if !errors.As(err, &conflict) || conflict.Status != http.StatusConflict {
		t.Fatalf("expected settlement conflict after preview changed: %v", err)
	}

	router, err := newAdminRouter(services, false)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(router)
	defer server.Close()

	if os.Getenv("BTYX_ADMIN_BASELINE_EXE") != "" {
		captureAdminParity(t, ctx, root, server.URL, services.Runtime.Config.Schema, username, password)
		return
	}

	command := exec.CommandContext(ctx, "node", "browser/verify-htmx.mjs")
	command.Dir = filepath.Join(root, "backend", "cmd", "admin", "tests")
	command.Env = append(os.Environ(),
		"BTYX_BROWSER_URL="+server.URL,
		"BTYX_BROWSER_USER="+username,
		"BTYX_BROWSER_PASSWORD="+password,
		"BTYX_BROWSER_REVIEWER="+reviewer.Username,
		"BTYX_BROWSER_FILE="+file.ID,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Edge browser verification: %v\n%s", err, output)
	}

	t.Logf("%s", output)
	wallet, err := services.Wallet(ctx, credit.UserID)
	if err != nil || wallet.AvailableFen != "875" {
		t.Fatalf("expected settled and adjusted balance of 875 fen: %v", err)
	}

	result, err := services.Assignment(ctx, review.UserID, review.ID)
	if err != nil || len(result.Submissions) != 2 {
		t.Fatalf("review submissions: %v", err)
	}

	for _, item := range result.Submissions {
		if item.Status != enum.SubmissionRejected {
			t.Fatalf("expected rejected platform %s, got %s", item.Platform, item.Status)
		}
	}

	result, err = services.Assignment(ctx, approval.UserID, approval.ID)
	if err != nil || result.Status != enum.AssignmentPendingSettlement || len(result.Submissions) != 2 {
		t.Fatalf("approved assignment status %s with %d submissions: %v", result.Status, len(result.Submissions), err)
	}

	for _, item := range result.Submissions {
		if item.Status != enum.SubmissionApproved {
			t.Fatalf("expected approved platform %s, got %s", item.Platform, item.Status)
		}
	}

	browserCreativeEvidence(t, ctx, services, account.UserID, member.UserID)
	for _, script := range []string{"smoke", "verify-task-layout", "verify-creative-evidence"} {
		t.Run(script, func(t *testing.T) {
			command := exec.CommandContext(ctx, "node", "browser/"+script+".mjs")
			command.Dir = filepath.Join(root, "backend", "cmd", "admin", "tests")
			command.Env = append(os.Environ(),
				"BTYX_ADMIN_URL="+server.URL,
				"BOOTSTRAP_ADMIN_USERNAME="+username,
				"BOOTSTRAP_ADMIN_PASSWORD="+password,
			)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("Edge browser verification: %v\n%s", err, output)
			}

			t.Logf("%s", output)
		})
	}
}

func browserEnrollmentFixture(t *testing.T, ctx context.Context, services *business.Service) {
	t.Helper()
	member := browserIdentityMember(t, ctx, services, "报名回归用户")
	front := browserIdentityPhoto(t, ctx, services, member, "enrollment-front")
	back := browserIdentityPhoto(t, ctx, services, member, "enrollment-back")
	err := services.Client.UserEnrollment.Create().
		SetUserID(member).
		SetApplicantNickname("报名回归用户").
		SetStatus(enum.EnrollmentPending).
		SetDouyin("dy-fixture").
		SetXhs("xhs-fixture").
		SetFrontFileID(front).
		SetBackFileID(back).
		SetIdentityCheckedAt(services.Now()).
		SetSubmittedAt(services.Now()).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func browserSettlementFixtures(t *testing.T, ctx context.Context, services *business.Service, credit task.AssignmentDTO) {
	t.Helper()

	nickname := "结算验收分发员"
	if err := services.UpdateProfile(ctx, credit.UserID, identity.UpdateProfileInput{Nickname: &nickname}); err != nil {
		t.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+p9lEAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "settlement-playback.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	screenshot, err := services.Client.FileObject.Create().
		SetOwnerID(credit.UserID).
		SetName("settlement-playback.png").
		SetPath(path).
		SetMediaType(enum.FilePNG).
		SetSize(int64(len(data))).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := services.Client.Submission.Update().
		Where(entsubmission.AssignmentIDEQ(credit.ID), entsubmission.PlatformEQ(enum.PlatformDouyin)).
		SetScreenshot(screenshot.ID).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := services.Client.Submission.Update().
		Where(entsubmission.AssignmentIDEQ(credit.ID), entsubmission.PlatformEQ(enum.PlatformXiaohongshu)).
		SetScreenshot("").
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := services.Client.Withdrawal.Create().
		SetUserID(credit.UserID).
		SetRequestKey(business.NewID()).
		SetStatus(enum.WithdrawalSuccess).
		SetAmountFen(decimal.NewFromInt(2050)).
		SetCreatedAt(time.Date(2026, 10, 31, 23, 55, 0, 0, beijing)).
		SetUpdatedAt(time.Date(2026, 11, 1, 0, 5, 0, 0, beijing)).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}
}

func browserAssignment(t *testing.T, ctx context.Context, services *business.Service, operatorID, title string, platforms []enum.Platform) task.AssignmentDTO {
	t.Helper()

	member, err := services.DemoLogin(ctx, business.NewID())
	if err != nil {
		t.Fatal(err)
	}

	if err := services.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentApproved).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := services.Consent(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := services.CreateTask(ctx, operatorID, task.TaskInput{
		Title:     title,
		Body:      "发布要求与验收素材",
		Project:   "浏览器测试项目",
		PriceFen:  "750",
		Platforms: platforms,
	})
	if err != nil {
		t.Fatal(err)
	}

	assignment, err := services.Claim(ctx, member.UserID, version.TaskID, business.NewID())
	if err != nil {
		t.Fatal(err)
	}

	for _, platform := range platforms {
		url := "https://v.douyin.com/test-work/"
		if platform == enum.PlatformXiaohongshu {
			url = "https://xhslink.cn/o/test-work"
		}

		if err := services.Submit(ctx, member.UserID, assignment.ID, submission.SubmitInput{
			Platform:   platform,
			URL:        url,
			Screenshot: "test-shot",
		}); err != nil {
			t.Fatal(err)
		}
	}

	return assignment
}

func browserCreativeEvidence(t *testing.T, ctx context.Context, s *business.Service, actor, member string) {
	t.Helper()
	when := s.Now().AddDate(0, 0, -10)
	taskID, err := s.Creative.Create(ctx, actor, creative.TaskInput{
		Title:        "双平台创作 · 点赞截图验收",
		Requirements: "提交已发布作品的点赞截图",
		Platforms:    []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu},
		MediaType:    enum.FilePNG,
		CreationFen:  "4000",
		StartsAt:     when.AddDate(0, 0, -1),
		EndsAt:       when.AddDate(0, 0, 1),
		RewardAt:     when.AddDate(0, 0, 4),
		PublishDays:  3,
		Tiers: []creative.Tier{{
			Likes:     3000,
			RewardFen: "26600",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	work, err := s.Client.CreativeWork.Create().
		SetTaskID(taskID).
		SetUserID(member).
		SetPublishFen(decimal.NewFromInt(750)).
		SetRevision(1).
		SetBusinessDate("2026-11-13").
		SetRequestKey(business.NewID()).
		SetStatus(enum.CreativeLinked).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, platform := range []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu} {
		name := "creative-" + string(platform) + ".png"
		filePath, err := filepath.Abs(filepath.Join("tests", "browser", "fixtures", name))
		if err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatal(err)
		}

		snapshot, err := s.Client.FileObject.Create().
			SetOwnerID(member).
			SetName(name).
			SetPath(filePath).
			SetMediaType(enum.FilePNG).
			SetSize(info.Size()).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		url := "https://v.douyin.com/creativeQC/"
		if platform == enum.PlatformXiaohongshu {
			url = "https://xhslink.cn/a/creativeQC"
		}

		err = s.Client.CreativeLink.Create().
			SetWorkID(work.ID).
			SetPlatform(platform).
			SetURL(url).
			SetScreenshot(snapshot.ID).
			SetCreatedAt(when).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
}
