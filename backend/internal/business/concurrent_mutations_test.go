package business

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent/agreement"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	submissions "baituber.cn/yanxuan/v2/internal/modules/submission"
	"github.com/shopspring/decimal"
)

func concurrentOperations(count int, operation func(int) error) []error {
	start := make(chan struct{})
	errors := make([]error, count)
	var waiting sync.WaitGroup
	for index := range count {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			<-start
			errors[index] = operation(index)
		}()
	}

	close(start)
	waiting.Wait()
	return errors
}

func TestConcurrentFirstLoginsCreateOneUserAndWallet(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	userIDs := make([]string, 8)
	for _, err := range concurrentOperations(len(userIDs), func(index int) error {
		login, err := service.DemoLogin(ctx, "concurrent-first-login")
		userIDs[index] = login.UserID
		return err
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, userID := range userIDs {
		if userID == "" || userID != userIDs[0] {
			t.Fatalf("concurrent login returned different users: %v", userIDs)
		}
	}

	count, err := service.Client.Wallet.Query().Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("expected one wallet: count=%d, err=%v", count, err)
	}
}

func TestConcurrentWithdrawalRetriesAndTransferCallbacksAreIdempotent(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	login, err := service.DemoLogin(ctx, "concurrent-wallet")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Client.Wallet.Update().
		Where(wallet.UserIDEQ(login.UserID)).
		SetAvailableFen(decimal.NewFromInt(10000)).
		Save(ctx); err != nil {
		t.Fatal(err)
	}

	withdrawalIDs := make([]string, 8)
	for _, err := range concurrentOperations(len(withdrawalIDs), func(index int) error {
		record, err := service.Withdraw(ctx, login.UserID, "8000", "same-withdrawal")
		withdrawalIDs[index] = record.ID
		return err
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, id := range withdrawalIDs {
		if id == "" || id != withdrawalIDs[0] {
			t.Fatalf("retries created different withdrawals: %v", withdrawalIDs)
		}
	}

	for _, err := range concurrentOperations(8, func(int) error {
		return service.ApplyTransferResult(ctx, withdrawalIDs[0], enum.WithdrawalSuccess, "same-provider-bill")
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	balance, err := service.Wallet(ctx, login.UserID)
	if err != nil || balance.AvailableFen != "2000" || balance.FrozenFen != "0" || balance.WithdrawnFen != "8000" {
		t.Fatalf("duplicate wallet effects: %+v, err=%v", balance, err)
	}

	count, err := service.Client.LedgerEntry.Query().Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("expected one freeze and one terminal ledger entry: count=%d, err=%v", count, err)
	}
}

func TestConcurrentPublishingKeepsSequentialImmutableVersions(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	input := TaskInput{
		Title:     "并发发布版本",
		Body:      "每次发布保留独立版本",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	}
	first, err := service.CreateTask(ctx, "admin", input)
	if err != nil {
		t.Fatal(err)
	}

	for _, err := range concurrentOperations(6, func(int) error {
		_, err := service.PublishVersion(ctx, "admin", first.TaskID, input)
		return err
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	currentTask, err := service.Client.Task.Get(ctx, first.TaskID)
	if err != nil {
		t.Fatal(err)
	}

	currentVersion, err := service.Client.TaskVersion.Get(ctx, currentTask.CurrentVersionID)
	if err != nil || currentVersion.Number != 7 {
		t.Fatalf("published version sequence lost: %+v, err=%v", currentVersion, err)
	}

	count, err := service.Client.TaskVersion.Query().Count(ctx)
	if err != nil || count != 7 {
		t.Fatalf("expected seven immutable versions: count=%d, err=%v", count, err)
	}
}

func TestConcurrentFirstAgreementPublishingKeepsOneActiveVersion(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := service.Client.Agreement.Delete().Exec(ctx); err != nil {
		t.Fatal(err)
	}

	for _, err := range concurrentOperations(6, func(index int) error {
		return service.PublishAgreement(ctx, "admin", fmt.Sprintf("协议%d", index), "合作条件")
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	count, err := service.Client.Agreement.Query().Where(agreement.ActiveEQ(true)).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("expected one active agreement: count=%d, err=%v", count, err)
	}

	count, err = service.Client.Agreement.Query().Count(ctx)
	if err != nil || count != 6 {
		t.Fatalf("concurrent publishing lost versions: count=%d, err=%v", count, err)
	}
}

func TestConcurrentReviewAndSettlementRetriesDoNotDuplicateCredit(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	login, err := service.DemoLogin(ctx, "concurrent-settlement")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := service.CreateTask(ctx, "admin", TaskInput{
		Title:     "并发结算",
		Body:      "只入账一次",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := service.Claim(ctx, login.UserID, version.TaskID, "concurrent-settlement-claim")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform:   enum.PlatformDouyin,
		URL:        "https://v.douyin.com/concurrent-settlement/",
		Screenshot: "verified-screenshot",
	}); err != nil {
		t.Fatal(err)
	}

	for _, err := range concurrentOperations(6, func(int) error {
		return service.ReviewAtRevision(ctx, "admin", assigned.ID, submissions.ReviewInput{
			Platform: enum.PlatformDouyin,
			Revision: 1,
			Approved: true,
		})
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	service.Now = func() time.Time {
		return time.Date(2026, 11, 23, 14, 0, 0, 0, time.FixedZone("CST", 8*3600))
	}
	batchIDs := make([]string, 6)
	for _, err := range concurrentOperations(len(batchIDs), func(index int) error {
		batch, err := service.Settle(ctx, "admin", SettlementInput{
			Through: "2026-11-23",
			Key:     "same-settlement",
		})
		batchIDs[index] = batch.ID
		return err
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, id := range batchIDs {
		if id == "" || id != batchIDs[0] {
			t.Fatalf("settlement retries returned different batches: %v", batchIDs)
		}
	}

	balance, err := service.Wallet(ctx, login.UserID)
	if err != nil || balance.AvailableFen != "750" {
		t.Fatalf("settlement duplicated credit: %+v, err=%v", balance, err)
	}

	count, err := service.Client.SettlementLine.Query().Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("settlement duplicated platform lines: count=%d, err=%v", count, err)
	}
}
