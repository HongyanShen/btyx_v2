package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	notices "baituber.cn/yanxuan/v2/internal/modules/notification"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"baituber.cn/yanxuan/v2/internal/testkit"
	entsql "entgo.io/ent/dialect/sql"
)

type countedSender struct {
	calls atomic.Int32
	err   error
}

func TestLockedNoticeJobsSkipConcurrentBatch(t *testing.T) {
	s := testkit.Service(t, "../../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	past := time.Now().Add(-time.Hour)
	statuses := []enum.JobStatus{enum.JobQueued, enum.JobRetry, enum.JobRunning}
	var pending []*ent.JobCreate
	for i := range 101 {
		pending = append(pending, s.Client.Job.Create().
			SetKind(enum.JobNotification).
			SetStatus(statuses[i%len(statuses)]).
			SetReferenceID(app.NewID()).
			SetRequestID(app.NewID()).
			SetCreatedAt(past.Add(time.Duration(i)*time.Second)).
			SetNextAt(past))
	}

	rows, err := s.Client.Job.CreateBulk(pending...).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, excluded := range []*ent.JobCreate{
		s.Client.Job.Create().SetKind(enum.JobTransfer).SetNextAt(past),
		s.Client.Job.Create().SetKind(enum.JobNotification).SetStatus(enum.JobDone).SetNextAt(past),
	} {
		if err := excluded.SetReferenceID(app.NewID()).SetCreatedAt(past.Add(-time.Hour)).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	first, err := s.Client.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer first.Rollback()
	batch, err := lockedNoticeJobs(ctx, first)
	if err != nil || len(batch) != 100 || batch[0].ID != rows[0].ID || batch[99].ID != rows[99].ID {
		t.Fatalf("incorrect first batch: count=%d err=%v", len(batch), err)
	}

	second, err := s.Client.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer second.Rollback()
	remaining, err := lockedNoticeJobs(ctx, second, entsql.WithLockAction(entsql.SkipLocked))
	if err != nil || len(remaining) != 1 || remaining[0].ID != rows[100].ID || remaining[0].RequestID != rows[100].RequestID || remaining[0].ReferenceID != rows[100].ReferenceID {
		t.Fatalf("second consumer did not skip the locked rows: count=%d err=%v", len(remaining), err)
	}
}

func (s *countedSender) SendCard(_ context.Context, openID string, card wxnotice.Card) (int, error) {
	if openID == "" || card.TemplateID != wxnotice.Settlement {
		return 47003, nil
	}

	s.calls.Add(1)
	return 0, s.err
}

func TestRiverSubscriptionDelivery(t *testing.T) {
	s := testkit.Service(t, "../../../..")
	ctx := context.Background()
	login, err := s.DemoLogin(ctx, "river-notices")
	if err != nil {
		t.Fatal(err)
	}

	sender := &countedSender{}
	client, err := startNoticeWorkers(ctx, s, sender)
	if err != nil {
		t.Fatal(err)
	}

	defer client.Stop(ctx)
	record := func(result enum.SubscriptionResult) {
		t.Helper()
		batch := notices.ChoiceBatch{
			Attempt:    app.NewID(),
			Scene:      "screenshots",
			ObservedAt: s.Now(),
			Choices: []notices.Choice{{
				TemplateID: wxnotice.Settlement,
				Result:     result,
			}},
		}
		for range 2 {
			if err := s.RecordChoices(ctx, login.UserID, batch); err != nil {
				t.Fatal(err)
			}
		}
	}
	send := func(amount string) *ent.Notification {
		t.Helper()
		ref := app.NewID()
		_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
			return true, s.Notify(ctx, tx, login.UserID, "分发结算已入账", "测试", ref, wxnotice.Credit(amount))
		})
		if err != nil {
			t.Fatal(err)
		}

		legacyCount, err := s.Client.Job.Query().Count(ctx)
		if err != nil || legacyCount != 0 {
			t.Fatalf("new notifications must go directly to River: legacy=%d err=%v", legacyCount, err)
		}

		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			rows, err := s.Client.Notification.Query().All(ctx)
			if err != nil {
				t.Fatal(err)
			}

			for _, n := range rows {
				if n.ReferenceID == ref && n.DeliveryState != enum.DeliveryPending && n.DeliveryState != enum.DeliverySending {
					return n
				}
			}

			time.Sleep(25 * time.Millisecond)
		}

		t.Fatal("River did not finish")
		return nil
	}
	record(enum.SubscriptionAccept)
	oversize := send("12345678901.23")
	if oversize.DeliveryState != enum.DeliveryFailed || oversize.ChoiceID != "" || sender.calls.Load() != 0 {
		t.Fatal("超限金额不得发送或占用订阅次数")
	}

	first := send("12.34")
	if first.DeliveryState != enum.DeliverySent || first.ChoiceID == "" {
		t.Fatal(first.DeliveryReason)
	}

	if n := send("12.34"); n.DeliveryState != enum.DeliverySkipped {
		t.Fatal("one consent used twice")
	}

	record(enum.SubscriptionAccept)
	record(enum.SubscriptionReject)
	if n := send("12.34"); n.DeliveryState != enum.DeliverySkipped {
		t.Fatal("delivered after rejection")
	}

	record(enum.SubscriptionAccept)
	sender.err = errors.New("transport interrupted")
	unknown := send("12.34")
	if unknown.DeliveryState != enum.DeliveryUnknown {
		t.Fatal("ambiguous delivery retried")
	}

	if err = s.Deliver(ctx, unknown.ID, "test", sender, false); err != nil {
		t.Fatal(err)
	}

	if sender.calls.Load() != 2 {
		t.Fatal("incorrect number of WeChat requests")
	}
}
