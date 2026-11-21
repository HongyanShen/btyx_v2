package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestConcurrentWithdrawalsCannotSpendFrozenFunds(t *testing.T) {
	services, first, _ := fundedWithdrawal(t)
	ctx := context.Background()
	record, err := services.Client.Withdrawal.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}

	var succeeded atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			if _, err := services.Withdraw(ctx, record.UserID, "500", fmt.Sprintf("concurrent-%d", index)); err == nil {
				succeeded.Add(1)
			}
		}(index)
	}

	wait.Wait()
	if succeeded.Load() != 1 {
		t.Fatalf("spent available funds %d times", succeeded.Load())
	}

	wallet, err := services.Wallet(ctx, record.UserID)
	if err != nil || wallet.AvailableFen != "0" || wallet.FrozenFen != "1500" {
		t.Fatalf("concurrent withdrawal balances: %+v, err=%v", wallet, err)
	}
}

func TestDuplicateFailedTransferRefundsExactlyOnce(t *testing.T) {
	services, withdrawal, _ := fundedWithdrawal(t)
	ctx := context.Background()
	for index := 0; index < 2; index++ {
		if err := services.ApplyTransferResult(ctx, withdrawal.ID, enum.WithdrawalFailed, "same-provider-id"); err != nil {
			t.Fatal(err)
		}
	}

	record, err := services.Client.Withdrawal.Get(ctx, withdrawal.ID)
	if err != nil {
		t.Fatal(err)
	}

	wallet, err := services.Wallet(ctx, record.UserID)
	if err != nil || wallet.AvailableFen != "1500" || wallet.FrozenFen != "0" || wallet.WithdrawnFen != "0" {
		t.Fatalf("duplicate refund balances: %+v, err=%v", wallet, err)
	}

	if err := services.ApplyTransferResult(ctx, withdrawal.ID, enum.WithdrawalSuccess, "same-provider-id"); err == nil {
		t.Fatal("failed terminal transfer was reversed")
	}
}
