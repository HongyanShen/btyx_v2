package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

type testGateway struct {
	submit func(context.Context, transfer.Request) (transfer.Result, error)
	query  func(context.Context, string) (transfer.Result, error)
}

func (gateway testGateway) Submit(ctx context.Context, request transfer.Request) (transfer.Result, error) {
	return gateway.submit(ctx, request)
}

func (gateway testGateway) Query(ctx context.Context, id string) (transfer.Result, error) {
	return gateway.query(ctx, id)
}

func fundedWithdrawal(t *testing.T) (*business.Service, business.WithdrawalDTO, string) {
	t.Helper()
	services := testkit.Service(t, "../../../..")
	ctx := context.Background()
	login, err := services.DemoLogin(ctx, "recovery-user")
	if err != nil {
		t.Fatal(err)
	}

	openID, err := services.WechatOpenID(ctx, login.UserID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = services.Client.Wallet.Update().SetAvailableFen(decimal.NewFromInt(1500)).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	withdrawal, err := services.Withdraw(ctx, login.UserID, "1000", "recover-original")
	if err != nil {
		t.Fatal(err)
	}

	return services, withdrawal, openID
}

func TestTransferCallbackCannotBeOverwrittenByWorkerResponse(t *testing.T) {
	services, withdrawal, openID := fundedWithdrawal(t)
	ctx := context.Background()
	gateway := testGateway{
		submit: func(ctx context.Context, request transfer.Request) (transfer.Result, error) {
			err := services.ApplyTransferResult(ctx, withdrawal.ID, enum.WithdrawalSuccess, "provider-original")
			return transfer.Result{
				Status:     enum.WithdrawalSubmitting,
				ProviderID: "provider-original",
				AmountFen:  request.AmountFen,
				OpenID:     request.OpenID,
			}, err
		},
	}
	if err := New(services, gateway).Tick(ctx); err != nil {
		t.Fatal(err)
	}

	record, err := services.Client.Withdrawal.Get(ctx, withdrawal.ID)
	if err != nil || record.Status != enum.WithdrawalSuccess {
		t.Fatalf("callback was overwritten: record=%v, err=%v", record, err)
	}

	wallet, err := services.Wallet(ctx, record.UserID)
	if err != nil || wallet.FrozenFen != "0" || wallet.WithdrawnFen != "1000" {
		t.Fatalf("incorrect terminal balances: %+v, err=%v, openID=%s", wallet, err, openID)
	}
}

func TestTransferTimeoutQueriesOriginalBillBeforeRetry(t *testing.T) {
	services, withdrawal, openID := fundedWithdrawal(t)
	ctx := context.Background()
	submits := 0
	queries := 0
	gateway := testGateway{
		submit: func(context.Context, transfer.Request) (transfer.Result, error) {
			submits++
			return transfer.Result{}, errors.New("network timeout")
		},
		query: func(_ context.Context, id string) (transfer.Result, error) {
			queries++
			if id != withdrawal.ID {
				t.Fatalf("query changed merchant bill: %s", id)
			}

			return transfer.Result{
				Status:     enum.WithdrawalSuccess,
				ProviderID: "provider-original",
				AmountFen:  "1000",
				OpenID:     openID,
			}, nil
		},
	}
	runner := New(services, gateway)
	if err := runner.Tick(ctx); err != nil {
		t.Fatal(err)
	}

	now := services.Now()
	services.Now = func() time.Time { return now.Add(time.Minute) }
	if err := runner.Tick(ctx); err != nil {
		t.Fatal(err)
	}

	if submits != 1 || queries != 1 {
		t.Fatalf("unsafe retry: submits=%d, queries=%d", submits, queries)
	}
}

func TestConfiguredGatewayResumesBlockedTransfer(t *testing.T) {
	services, withdrawal, _ := fundedWithdrawal(t)
	ctx := context.Background()
	if err := New(services, nil).Tick(ctx); err != nil {
		t.Fatal(err)
	}

	submits := 0
	gateway := testGateway{
		submit: func(_ context.Context, request transfer.Request) (transfer.Result, error) {
			submits++
			if request.ID != withdrawal.ID {
				t.Fatal("resuming configuration changed the merchant bill")
			}

			return transfer.Result{
				Status:     enum.WithdrawalSubmitting,
				ProviderID: "provider-original",
				AmountFen:  request.AmountFen,
				OpenID:     request.OpenID,
			}, nil
		},
	}
	if err := New(services, gateway).Tick(ctx); err != nil {
		t.Fatal(err)
	}

	if submits != 1 {
		t.Fatal("configured worker did not resume the blocked transfer")
	}
}

func TestCrashBeforeDispatchResubmitsOnlyOriginalBillAfterNotFound(t *testing.T) {
	services, withdrawal, _ := fundedWithdrawal(t)
	ctx := context.Background()
	_, err := services.Client.Withdrawal.UpdateOneID(withdrawal.ID).
		SetStatus(enum.WithdrawalUnknown).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = services.Client.Job.Update().SetAttempts(1).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	queries := 0
	submits := 0
	gateway := testGateway{
		query: func(_ context.Context, id string) (transfer.Result, error) {
			queries++
			return transfer.Result{}, transfer.ErrNotFound
		},
		submit: func(_ context.Context, request transfer.Request) (transfer.Result, error) {
			submits++
			if queries != 1 || request.ID != withdrawal.ID {
				t.Fatal("resubmission skipped query or changed the original bill")
			}

			return transfer.Result{
				Status:     enum.WithdrawalSubmitting,
				ProviderID: "provider-original",
				AmountFen:  request.AmountFen,
				OpenID:     request.OpenID,
			}, nil
		},
	}
	if err := New(services, gateway).Tick(ctx); err != nil {
		t.Fatal(err)
	}

	if submits != 1 {
		t.Fatal("original bill was never recovered")
	}
}
