package task

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/job"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/noticequeue"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/riverqueue/river"
)

// Drain old writers' rows on startup. Wait for outstanding locks before declaring the drain complete.
func migrateLegacyNotices(ctx context.Context, s *business.Service) error {
	for {
		count, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (int, error) {
			rows, err := lockedNoticeJobs(ctx, tx)
			if err != nil || len(rows) == 0 {
				return 0, err
			}

			native, err := tx.SQLTx()
			if err != nil {
				return 0, err
			}

			for _, row := range rows {
				args := noticequeue.Args{
					ID:        row.ReferenceID,
					RequestID: row.RequestID,
				}
				if args.RequestID == "" {
					args.RequestID = row.ID
				}

				opts := &river.InsertOpts{}
				if row.NextAt.After(s.Now()) {
					opts.ScheduledAt = row.NextAt
				}

				if _, err := s.NoticeQueue.InsertTx(ctx, native, args, opts); err != nil {
					return 0, err
				}

				if err := tx.Job.UpdateOneID(row.ID).SetStatus(enum.JobDone).Exec(ctx); err != nil {
					return 0, err
				}
			}

			return len(rows), nil
		})
		if err != nil || count < 100 {
			return err
		}
	}
}

func lockedNoticeJobs(ctx context.Context, tx *ent.Tx, opts ...entsql.LockOption) ([]*ent.Job, error) {
	return tx.Job.Query().
		Where(
			job.KindEQ(enum.JobNotification),
			job.StatusIn(enum.JobQueued, enum.JobRetry, enum.JobRunning, enum.JobBlockedConfig),
		).
		Order(ent.Asc(job.FieldCreatedAt), ent.Asc(job.FieldID)).
		Limit(100).
		ForUpdate(opts...).
		Select(job.FieldID, job.FieldReferenceID, job.FieldRequestID, job.FieldNextAt).
		All(ctx)
}
