package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/noticequeue"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNoticeTransactionAndTrace(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdk.NewTracerProvider(sdk.WithSpanProcessor(recorder)))
	defer otel.SetTracerProvider(previous)
	s := testkit.Service(t, "../../../..")
	ctx, root := otel.Tracer("test").Start(context.Background(), "request")
	defer root.End()
	rollback := errors.New("rollback")
	const sensitive = "private-token-never-export"
	queueCount := func() int {
		t.Helper()
		var n int
		if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM river_job WHERE kind='wechat_subscription'").Scan(&n); err != nil {
			t.Fatal(err)
		}

		return n
	}
	login, err := s.DemoLogin(ctx, "trace-notice")
	if err != nil {
		t.Fatal(err)
	}

	uid := login.UserID
	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		if err := s.Notify(ctx, tx, uid, "test", sensitive, app.NewID(), wxnotice.Credit("1.23")); err != nil {
			return false, err
		}

		if queueCount() != 0 {
			t.Fatal("uncommitted queue entry visible")
		}

		return false, rollback
	})
	if !errors.Is(err, rollback) || queueCount() != 0 {
		t.Fatalf("rollback: %v", err)
	}

	if n, err := s.Client.Notification.Query().Count(ctx); err != nil || n != 0 {
		t.Fatalf("rollback left notifications: %d %v", n, err)
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		if err := s.Notify(ctx, tx, uid, "inbox", sensitive, app.NewID()); err != nil {
			return false, err
		}

		return true, s.Notify(ctx, tx, uid, "wechat", sensitive, app.NewID(), wxnotice.Credit("1.23"))
	})
	if err != nil || queueCount() != 1 {
		t.Fatalf("commit: %v", err)
	}

	var parent string
	if err := s.DB.QueryRowContext(ctx, "SELECT args->>'traceparent' FROM river_job WHERE kind=$1", noticequeue.Args{}.Kind()).Scan(&parent); err != nil {
		t.Fatal(err)
	}

	client, err := startNoticeWorkers(context.Background(), s, &countedSender{})
	if err != nil {
		t.Fatal(err)
	}

	defer client.Stop(context.Background())
	deadline := time.Now().Add(15 * time.Second)
	for {
		done, err := s.Client.Notification.Query().Where(notification.DeliveryStateEQ(enum.DeliverySkipped)).Exist(ctx)
		if err != nil || time.Now().After(deadline) {
			t.Fatalf("worker did not process committed job: %v", err)
		}

		if done {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	if err := client.Client.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	var sqlSpan, producer, linked bool
	for _, span := range recorder.Ended() {
		if span.Name() == "enqueue wechat_subscription" || span.Name() == "wechat_subscription" {
			attrs := attribute.NewSet(span.Attributes()...)
			id, ok := attrs.Value("user.id")
			if !ok || id.AsString() != uid {
				t.Fatalf("%s missing recipient ID", span.Name())
			}
		}

		if span.Name() == "enqueue wechat_subscription" {
			producer = true
		}

		if span.Name() == "wechat_subscription" && span.SpanContext().TraceID() == root.SpanContext().TraceID() && strings.Contains(parent, span.Parent().SpanID().String()) {
			linked = true
		}

		for _, a := range span.Attributes() {
			value := a.Value.Emit()
			if strings.Contains(value, sensitive) {
				t.Fatal("exported private SQL")
			}

			if a.Key == "db.query.text" {
				if strings.Contains(value, "'wechat_subscription'") {
					t.Fatal("exported SQL literal")
				}

				if span.Name() == "postgresql INSERT" && span.SpanContext().TraceID() == root.SpanContext().TraceID() &&
					strings.Contains(value, "notifications") && strings.Contains(value, "$1") {
					sqlSpan = true
				}
			}
		}
	}

	if !sqlSpan || !producer || !linked {
		t.Fatal("missing SQL, producer or linked consumer span")
	}
}

func TestLegacyNoticesDrainOnceAndKeepSchedule(t *testing.T) {
	s := testkit.Service(t, "../../../..")
	ctx := context.Background()
	future := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	var rows []*ent.JobCreate
	for i := range 103 {
		status := enum.JobRetry
		when := s.Now().Add(-time.Hour)
		if i == 102 {
			status = enum.JobRunning
			when = future
		}

		rows = append(rows, s.Client.Job.Create().SetKind(enum.JobNotification).SetStatus(status).SetReferenceID(app.NewID()).SetNextAt(when))
	}

	created, err := s.Client.Job.CreateBulk(rows...).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := migrateLegacyNotices(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM river_job WHERE kind='wechat_subscription'").Scan(&count); err != nil || count != 103 {
		t.Fatalf("count=%d err=%v", count, err)
	}

	var when time.Time
	var state, origin string
	if err := s.DB.QueryRowContext(ctx, "SELECT scheduled_at,state,args->>'request_id' FROM river_job WHERE args->>'notification_id'=$1", created[102].ReferenceID).Scan(&when, &state, &origin); err != nil || !when.Equal(future) || state != "scheduled" || origin != created[102].ID {
		t.Fatalf("scheduled=%s state=%s origin=%s err=%v", when, state, origin, err)
	}
}
