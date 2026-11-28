package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/job"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

func TestJobsPersistOriginRequestIDAcrossWorkerRetries(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	var output bytes.Buffer
	requestContext := logging.NewRequest(logging.WithLogger(context.Background(), logging.New(&output, "miniapi")))
	requestID := logging.RequestID(requestContext)
	login, err := services.DemoLogin(requestContext, "job-log-origin")
	if err != nil {
		t.Fatal(err)
	}

	_, err = services.Client.Wallet.Update().SetAvailableFen(decimal.NewFromInt(1500)).Save(requestContext)
	if err != nil {
		t.Fatal(err)
	}

	withdrawal, err := services.Withdraw(requestContext, login.UserID, "1000", "logging-request")
	if err != nil {
		t.Fatal(err)
	}

	record, err := services.Client.Job.Query().Where(job.ReferenceIDEQ(withdrawal.ID)).Only(requestContext)
	if err != nil || record.RequestID != requestID {
		t.Fatalf("outbox lost origin request ID: record=%v err=%v", record, err)
	}

	observed := 0
	checkContext := func(ctx context.Context) error {
		observed++
		if logging.RequestID(ctx) != requestID {
			t.Errorf("worker attempt lost persisted request ID: %s", logging.RequestID(ctx))
		}

		logging.Info(ctx).Msg("transfer attempt")

		return errors.New("secret-provider-token")
	}
	gateway := testGateway{
		submit: func(ctx context.Context, request transfer.Request) (transfer.Result, error) {
			return transfer.Result{}, checkContext(ctx)
		},
		query: func(ctx context.Context, id string) (transfer.Result, error) {
			return transfer.Result{}, checkContext(ctx)
		},
	}
	workerContext := logging.NewRequest(logging.WithLogger(context.Background(), logging.New(&output, "worker")))
	runner := New(services, gateway)
	if err := runner.Tick(workerContext); err != nil {
		t.Fatal(err)
	}

	services.Now = func() time.Time {
		return time.Now().Add(time.Minute)
	}
	if err := runner.Tick(workerContext); err != nil {
		t.Fatal(err)
	}

	if observed != 2 || strings.Contains(output.String(), "secret-provider-token") {
		t.Fatalf("retry correlation or error privacy failed: attempts=%d logs=%s", observed, output.String())
	}

	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var entry struct {
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.RequestID != requestID {
			t.Fatalf("business log lost correlation: %s", line)
		}
	}

	_, err = app.Transaction(requestContext, services.Runtime, func(tx *ent.Tx) (bool, error) {
		return true, services.Notify(requestContext, tx, login.UserID, "test", "test", withdrawal.ID, wxnotice.Credit("10.00"))
	})
	if err != nil {
		t.Fatal(err)
	}

	var origin string
	err = services.DB.QueryRowContext(requestContext, `SELECT args->>'request_id' FROM river_job WHERE kind='wechat_subscription' AND args->>'notification_id' IN (SELECT id FROM notifications WHERE title='test')`).Scan(&origin)
	if err != nil || origin != requestID {
		t.Fatalf("River lost request ID: origin=%s err=%v", origin, err)
	}
}
