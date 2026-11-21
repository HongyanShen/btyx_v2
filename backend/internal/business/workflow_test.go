package business

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
)

type workflowIdentityCheck struct{}

func (workflowIdentityCheck) Check(context.Context, string, string) error {
	return nil
}

// These tests exercise real PostgreSQL constraints and transactions.
func testService(t *testing.T) *Service {
	t.Helper()
	s, cleanup, err := OpenTest(context.Background(), "../../../.env")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(cleanup)
	return s
}

func TestWorkflowPriceSnapshotPlatformReviewSettlementAndWithdrawal(t *testing.T) {
	s := testService(t)
	s.EnrollmentService.Verifier = workflowIdentityCheck{}
	ctx := context.Background()
	u, err := s.DemoLogin(ctx, "workflow-user")
	if err != nil {
		t.Fatal(err)
	}

	uid := u.UserID
	var photos []string
	for range 2 {
		photo, err := s.Client.FileObject.Create().
			SetOwnerID(uid).
			SetName("identity.jpg").
			SetBucket("test").
			SetObjectKey(NewID()).
			SetStorage(enum.FileStorageTOS).
			SetMediaType(enum.FileJPEG).
			SetSize(1024).
			SetVerifiedAt(s.Now()).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		photos = append(photos, photo.ID)
	}

	terms, err := s.CurrentAgreement(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Apply(ctx, uid, ApplyInput{
		DocumentInput: identity.DocumentInput{
			Name:        "测试分发员",
			Identity:    "990000199001010017",
			FrontFileID: photos[0],
			BackFileID:  photos[1],
		},
		Phone:       "13800000000",
		Douyin:      "demo-dy",
		XHS:         "demo-xhs",
		Tags:        []string{"时尚"},
		AgreementID: terms.ID,
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ReviewEnrollment(ctx, "admin", uid, true, "", []string{"时尚"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := s.Consent(ctx, uid); err != nil {
		t.Fatal(err)
	}

	v1, err := s.CreateTask(ctx, "admin", TaskInput{
		Title:     "首个营销任务",
		Body:      "发布指定内容",
		PriceFen:  "750",
		Platforms: []enum.Platform{"dy", "xhs"},
		Tags:      []string{"时尚"},
	})
	if err != nil {
		t.Fatal(err)
	}

	a, err := s.Claim(ctx, uid, v1.TaskID, "claim-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.PublishVersion(ctx, "admin", v1.TaskID, TaskInput{
		Title:     "新版本",
		Body:      "新要求",
		PriceFen:  "900",
		Platforms: []enum.Platform{"dy", "xhs"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Submit(ctx, uid, a.ID, SubmitInput{
		Platform:   "dy",
		URL:        "https://v.douyin.com/abc123/",
		Screenshot: "test-shot",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Submit(ctx, uid, a.ID, SubmitInput{
		Platform:   "xhs",
		URL:        "https://xhslink.cn/o/abc123",
		Screenshot: "test-shot-2",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ReviewAtRevision(ctx, "admin", a.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ReviewAtRevision(ctx, "admin", a.ID, submission.ReviewInput{
		Platform: enum.PlatformXiaohongshu,
		Revision: 1,
		Approved: false,
		Reason:   "缺少指定话题",
	}); err != nil {
		t.Fatal(err)
	}

	d, err := s.Assignment(ctx, uid, a.ID)
	if err != nil {
		t.Fatal(err)
	}

	if d.PriceFen != "750" || d.VersionID != v1.ID {
		t.Fatalf("snapshot changed: %+v", d)
	}

	if d.Status != "rejected" {
		t.Fatalf("got status %s", d.Status)
	}

	if err := s.Submit(ctx, uid, a.ID, SubmitInput{
		Platform:   "xhs",
		URL:        "https://xhslink.cn/o/abc123",
		Screenshot: "test-shot-3",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ReviewAtRevision(ctx, "admin", a.ID, submission.ReviewInput{
		Platform: enum.PlatformXiaohongshu,
		Revision: 2,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	d, _ = s.Assignment(ctx, uid, a.ID)
	if d.Status != "pending_settlement" {
		t.Fatalf("got %s", d.Status)
	}

	if _, err := s.Settle(ctx, "admin", SettlementInput{
		Through: "2026-11-23",
		Key:     "too-early",
	}); err == nil {
		t.Fatal("settled before the weekly due date")
	}

	s.Now = func() time.Time {
		return time.Date(2026, 11, 23, 14, 0, 0, 0, time.FixedZone("CST", 8*3600))
	}
	b, err := s.Settle(ctx, "admin", SettlementInput{
		Through: "2026-11-23",
		Key:     "settlement-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	if b.AmountFen != "1500" {
		t.Fatalf("settled %s", b.AmountFen)
	}

	_, err = s.Settle(ctx, "admin", SettlementInput{
		Through: "2026-11-23",
		Key:     "settlement-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	w, err := s.Wallet(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}

	if w.AvailableFen != "1500" {
		t.Fatalf("duplicate credit: %+v", w)
	}

	d, _ = s.Assignment(ctx, uid, a.ID)
	if d.Status != "settled" {
		t.Fatalf("got %s", d.Status)
	}

	wd, err := s.Withdraw(ctx, uid, "1000", "withdraw-1")
	if err != nil {
		t.Fatal(err)
	}

	wd2, err := s.Withdraw(ctx, uid, "1000", "withdraw-1")
	if err != nil || wd2.ID != wd.ID {
		t.Fatalf("idempotency failed %v", err)
	}

	w, _ = s.Wallet(ctx, uid)
	if w.AvailableFen != "500" || w.FrozenFen != "1000" {
		t.Fatalf("got %+v", w)
	}

	if err := s.ApplyTransferResult(ctx, wd.ID, "success", "wx-1"); err != nil {
		t.Fatal(err)
	}

	if err := s.ApplyTransferResult(ctx, wd.ID, "success", "wx-1"); err != nil {
		t.Fatal(err)
	}

	w, _ = s.Wallet(ctx, uid)
	if w.AvailableFen != "500" || w.FrozenFen != "0" || w.WithdrawnFen != "1000" {
		t.Fatalf("got %+v", w)
	}
}

func TestConcurrentClaimsCannotExceedDailyQuota(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	u, err := s.DemoLogin(ctx, "concurrent")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.PrepareTestUser(ctx, u.UserID); err != nil {
		t.Fatal(err)
	}

	v, err := s.CreateTask(ctx, "admin", TaskInput{
		Title:     "并发领取",
		Body:      "要求",
		PriceFen:  "750",
		Platforms: []enum.Platform{"dy"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Claim(ctx, u.UserID, v.TaskID, NewID())
			results <- e
		}()
	}

	wg.Wait()
	close(results)
	n := 0
	for e := range results {
		if e == nil {
			n++
		}
	}

	if n != 1 {
		t.Fatalf("%d concurrent claims succeeded", n)
	}
}

func TestSecretsNeverIncludedInExampleConfig(t *testing.T) {
	b, err := os.ReadFile("../../../.env.example")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(b), "Pg18_") {
		t.Fatal("real password leaked")
	}
}
