package task

import (
	"context"
	"database/sql"
	"time"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatmini"
	"baituber.cn/yanxuan/v2/internal/modules/notification"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/noticequeue"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"go.opentelemetry.io/otel/attribute"
)

type noticeClient struct {
	*river.Client[*sql.Tx]
	listener *pgxpool.Pool
}

func (c *noticeClient) Stop(ctx context.Context) error {
	if err := c.Client.Stop(ctx); err != nil {
		return err
	}

	c.listener.Close()
	return nil
}

type noticeWorker struct {
	river.WorkerDefaults[noticequeue.Args]
	services *business.Service
	sender   notification.Sender
}

func (w *noticeWorker) Work(ctx context.Context, job *river.Job[noticequeue.Args]) (err error) {
	ctx, span := telemetry.StartJob(ctx, job.Args.TraceParent, job.Kind, job.Attempt)
	defer func() { telemetry.Finish(span, err) }()
	span.SetAttributes(attribute.Int64("job.queue_wait_ms", max(0, time.Since(job.ScheduledAt).Milliseconds())), attribute.String("request_id", job.Args.RequestID))
	ctx = logging.WithRequestID(ctx, job.Args.RequestID)
	n, err := w.services.Client.Notification.Get(ctx, job.Args.ID)
	if err != nil {
		return err
	}

	telemetry.SetUser(ctx, n.UserID, enum.RoleUser)
	if n.DeliveryState != enum.DeliveryPending && n.DeliveryState != enum.DeliverySending {
		return nil
	}

	current, err := w.services.ReminderCurrent(ctx, n)
	if err != nil {
		return err
	}

	if !current {
		return w.services.Client.Notification.UpdateOne(n).SetDeliveryState(enum.DeliverySkipped).SetDeliveryReason("任务状态已变化或提醒已过期").Exec(ctx)
	}

	openID, err := w.services.WechatOpenID(ctx, n.UserID)
	if err != nil {
		if job.Attempt >= job.MaxAttempts {
			return w.services.Client.Notification.UpdateOne(n).SetDeliveryState(enum.DeliveryFailed).SetDeliveryReason("无法取得微信收件身份").Exec(ctx)
		}

		return err
	}

	return w.services.Deliver(ctx, n.ID, openID, w.sender, job.Attempt >= job.MaxAttempts)
}

type reminderArgs struct {
	RequestID string `json:"request_id"`
}

func (reminderArgs) Kind() string {
	return "marketing_submission_reminders"
}

type reminderWorker struct {
	river.WorkerDefaults[reminderArgs]
	services *business.Service
}

func (w *reminderWorker) Work(ctx context.Context, job *river.Job[reminderArgs]) (err error) {
	ctx, span := telemetry.StartJob(ctx, "", job.Kind, job.Attempt)
	defer func() { telemetry.Finish(span, err) }()
	return w.services.CreateReminders(logging.WithRequestID(ctx, job.Args.RequestID))
}

func StartNotices(ctx context.Context, s *business.Service) (*noticeClient, error) {
	return startNoticeWorkers(ctx, s, wechatmini.NewPhoneClient(s.Config.WechatAppID, s.Config.WechatAppSecret))
}

func startNoticeWorkers(ctx context.Context, s *business.Service, sender notification.Sender) (*noticeClient, error) {
	if err := migrateLegacyNotices(ctx, s); err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(s.Config.DSN(true))
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = 1
	listener, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &noticeWorker{
		services: s,
		sender:   sender,
	})
	river.AddWorker(workers, &reminderWorker{services: s})
	periodic := river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) { return reminderArgs{RequestID: business.NewID()}, nil }, &river.PeriodicJobOpts{RunOnStart: true})
	client, err := river.NewClient(riverdatabasesql.NewWithPgxListener(s.DB, listener), &river.Config{
		Schema:            s.Config.Schema,
		FetchPollInterval: time.Minute,
		Queues:            map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers:           workers,
		PeriodicJobs:      []*river.PeriodicJob{periodic},
	})
	if err != nil {
		listener.Close()
		return nil, err
	}

	if err = client.Start(ctx); err != nil {
		listener.Close()
		return nil, err
	}

	return &noticeClient{
		Client:   client,
		listener: listener,
	}, nil
}
