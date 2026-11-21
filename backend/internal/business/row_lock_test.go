package business

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	submissions "baituber.cn/yanxuan/v2/internal/modules/submission"
	"github.com/shopspring/decimal"
)

func waitForBlockedTransaction(t *testing.T, ctx context.Context, service *Service, transaction *sql.Tx) {
	t.Helper()

	var blockerPID int
	if err := transaction.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}

	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		var blocked bool
		err := service.DB.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid))
		)`, blockerPID).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}

		if blocked {
			return
		}

		select {
		case <-deadline.C:
			t.Fatal("business transaction did not wait for the locked row")
		case <-ticker.C:
		}
	}
}

func TestWithdrawalChecksBalanceAfterWalletRowLock(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	login, err := service.DemoLogin(ctx, "wallet-row-lock")
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Client.Wallet.Update().
		Where(wallet.UserIDEQ(login.UserID)).
		SetAvailableFen(decimal.NewFromInt(10000)).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	transaction, err := service.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	defer transaction.Rollback()
	_, err = transaction.ExecContext(ctx, "UPDATE wallets SET available_fen = 5000 WHERE user_id = $1", login.UserID)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := service.Withdraw(ctx, login.UserID, "8000", "locked-wallet")
		result <- err
	}()

	waitForBlockedTransaction(t, ctx, service, transaction)
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-result; err == nil {
		t.Fatal("withdrawal used the balance read before acquiring the wallet row lock")
	}

	balance, err := service.Wallet(ctx, login.UserID)
	if err != nil || balance.AvailableFen != "5000" || balance.FrozenFen != "0" {
		t.Fatalf("wallet changed despite insufficient current balance: %+v, err=%v", balance, err)
	}

	count, err := service.Client.Withdrawal.Query().Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("failed withdrawal left records: count=%d, err=%v", count, err)
	}
}

func TestReviewChecksRevisionAfterAssignmentRowLock(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	login, err := service.DemoLogin(ctx, "review-row-lock")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareTestUser(ctx, login.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := service.CreateTask(ctx, "admin", TaskInput{
		Title:     "行锁审核保护",
		Body:      "等待新的提交版本",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := service.Claim(ctx, login.UserID, version.TaskID, "locked-review-claim")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Submit(ctx, login.UserID, assigned.ID, SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/locked-review/",
	}); err != nil {
		t.Fatal(err)
	}

	transaction, err := service.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, "SELECT id FROM assignments WHERE id = $1 FOR UPDATE", assigned.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := transaction.ExecContext(ctx, "UPDATE submissions SET revision = 2 WHERE assignment_id = $1", assigned.ID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- service.ReviewAtRevision(ctx, "admin", assigned.ID, submissions.ReviewInput{
			Platform: enum.PlatformDouyin,
			Revision: 1,
			Approved: true,
		})
	}()

	waitForBlockedTransaction(t, ctx, service, transaction)
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-result; err == nil {
		t.Fatal("review approved a revision read before acquiring the assignment row lock")
	}

	record, err := service.Client.Submission.Query().
		Where(submission.AssignmentIDEQ(assigned.ID)).
		Only(ctx)
	if err != nil || record.Revision != 2 || record.Status != enum.SubmissionPending {
		t.Fatalf("unseen revision was modified: %+v, err=%v", record, err)
	}
}
