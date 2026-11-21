package business

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"github.com/shopspring/decimal"
)

func TestDatabaseEnforcesAppendOnlyLedgerAndNonnegativeBalance(t *testing.T) {
	services := testService(t)
	ctx := context.Background()
	login, err := services.DemoLogin(ctx, "ledger-integrity")
	if err != nil {
		t.Fatal(err)
	}

	entry, err := services.Client.LedgerEntry.Create().
		SetUserID(login.UserID).
		SetBusinessKey("integrity-original").
		SetKind(enum.LedgerSettlementCredit).
		SetAmountFen(decimal.NewFromInt(1000)).
		SetReferenceID("test-settlement").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.Client.LedgerEntry.UpdateOneID(entry.ID).SetReason("rewrite history").Save(ctx); err == nil {
		t.Fatal("database allowed ledger mutation")
	}

	if err := services.Client.LedgerEntry.DeleteOneID(entry.ID).Exec(ctx); err == nil {
		t.Fatal("database allowed ledger deletion")
	}

	if _, err := services.Client.Wallet.Update().SetAvailableFen(decimal.NewFromInt(-1)).Save(ctx); err == nil {
		t.Fatal("database allowed a negative available balance")
	}

	if err := services.AdjustLedger(ctx, "admin", entry.ID, "100", "核对后补差", "adjust-integrity"); err != nil {
		t.Fatal(err)
	}

	if err := services.AdjustLedger(ctx, "admin", entry.ID, "100", "核对后补差", "adjust-integrity"); err != nil {
		t.Fatal(err)
	}

	wallet, err := services.Wallet(ctx, login.UserID)
	if err != nil || wallet.AvailableFen != "100" {
		t.Fatalf("adjustment retry changed balance twice: %+v, err=%v", wallet, err)
	}

	if err := services.AdjustLedger(ctx, "admin", entry.ID, "200", "核对后补差", "adjust-integrity"); err == nil {
		t.Fatal("same idempotency key accepted a changed adjustment amount")
	}

	if err := services.AdjustLedger(ctx, "admin", entry.ID, "1e2", "科学计数输入", "adjust-exponent"); err == nil {
		t.Fatal("adjustment accepted a non-integer money representation")
	}
}
