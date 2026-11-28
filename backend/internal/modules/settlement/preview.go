package settlement

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/shopspring/decimal"
)

type SettlementPreview struct {
	Through   string                  `json:"through"`
	AmountFen string                  `json:"amount_fen"`
	Count     int64                   `json:"count"`
	Lines     []SettlementPreviewLine `json:"lines"`
}

type SettlementPreviewLine struct {
	Creative     bool          `json:"creative,omitempty"`
	SubmissionID string        `json:"submission_id"`
	AssignmentID string        `json:"assignment_id"`
	UserID       string        `json:"user_id"`
	Platform     enum.Platform `json:"platform"`
	AmountFen    string        `json:"amount_fen"`
	EligibleDate string        `json:"eligible_date"`
	revision     string
}

func settlementPreview(ctx context.Context, client *ent.Client, through string, now time.Time, exactPeriod bool) (SettlementPreview, error) {
	out := SettlementPreview{
		Through:   through,
		AmountFen: "0",
		Lines:     []SettlementPreviewLine{},
	}
	if !app.ValidDate(through) {
		return out, app.Invalid("结算日期无效")
	}

	query := client.Submission.Query().
		Where(
			submission.StatusIn(enum.SubmissionApproved, enum.SubmissionWaived),
			submission.SettlementIDEQ(""),
			submission.ScreenshotNEQ(""),
			submission.EligibleDateNEQ(""),
			submission.EligibleDateLTE(through),
			submission.EligibleDateLTE(app.BusinessDay(now)),
			submission.Or(submission.SettlementEligibleAtIsNil(), submission.SettlementEligibleAtLTE(now)),
			submission.VisibilityIn(enum.VisibilityManualVerified, enum.VisibilityVisible),
		)
	if exactPeriod {
		query.Where(submission.EligibleDateEQ(through))
	}

	rows, err := query.Limit(1001).All(ctx)
	if err != nil {
		return out, err
	}

	if len(rows) > 1000 {
		return out, app.Conflict("本批超过1000条，请缩小结算范围")
	}

	total := decimal.Zero
	for _, platformSubmission := range rows {
		assignmentRecord, err := client.Assignment.Get(ctx, platformSubmission.AssignmentID)
		if err != nil {
			return out, err
		}

		total = total.Add(assignmentRecord.PriceFen)
		out.Lines = append(out.Lines, SettlementPreviewLine{
			SubmissionID: platformSubmission.ID,
			AssignmentID: assignmentRecord.ID,
			UserID:       assignmentRecord.UserID,
			Platform:     platformSubmission.Platform,
			AmountFen:    assignmentRecord.PriceFen.String(),
			EligibleDate: platformSubmission.EligibleDate,
			revision:     settlementRevision(platformSubmission, assignmentRecord),
		})
	}

	extra, err := pendingCreative(ctx, client, through, now, exactPeriod)
	if err != nil {
		return out, err
	}

	for _, row := range extra {
		total = total.Add(row.AmountFen)
		out.Lines = append(out.Lines, SettlementPreviewLine{
			Creative:     true,
			SubmissionID: row.ID,
			AssignmentID: row.WorkID,
			UserID:       row.UserID,
			AmountFen:    row.AmountFen.String(),
			EligibleDate: app.BusinessDay(row.EligibleAt),
			revision:     creativeRevision(row),
		})
	}

	if len(out.Lines) > 1000 {
		return out, app.Conflict("本批超过1000条，请缩小结算范围")
	}

	out.Count = int64(len(out.Lines))
	out.AmountFen = total.String()
	return out, nil
}

func (s *Service) PreviewSettlement(ctx context.Context, through string) (SettlementPreview, error) {
	return settlementPreview(ctx, s.Client, through, s.Now(), false)
}
