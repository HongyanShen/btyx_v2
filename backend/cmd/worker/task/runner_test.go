package task

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

func TestMissingTransferConfigurationKeepsFundsFrozen(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	login, err := service.DemoLogin(ctx, "worker-funds")
	if err != nil {
		t.Fatal(err)
	}

	userWallet, err := service.Client.Wallet.Query().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Client.Wallet.UpdateOneID(userWallet.ID).
		SetAvailableFen(decimal.NewFromInt(1500)).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	withdrawal, err := service.Withdraw(ctx, login.UserID, "1000", "worker-withdrawal")
	if err != nil {
		t.Fatal(err)
	}

	runner := New(service, nil)
	if err := runner.Tick(ctx); err != nil {
		t.Fatal(err)
	}

	record, err := service.Client.Withdrawal.Get(ctx, withdrawal.ID)
	if err != nil {
		t.Fatal(err)
	}

	if record.Status != enum.WithdrawalBlockedConfig {
		t.Fatalf("unconfigured transfer became %s", record.Status)
	}

	wallet, err := service.Wallet(ctx, login.UserID)
	if err != nil {
		t.Fatal(err)
	}

	if wallet.AvailableFen != "500" || wallet.FrozenFen != "1000" || wallet.WithdrawnFen != "0" {
		t.Fatalf("funds changed without provider confirmation: %+v", wallet)
	}
}
