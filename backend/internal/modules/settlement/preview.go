package settlement

import (
	"context"

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
	SubmissionID string        `json:"submission_id"`
	AssignmentID string        `json:"assignment_id"`
	UserID       string        `json:"user_id"`
	Platform     enum.Platform `json:"platform"`
	AmountFen    string        `json:"amount_fen"`
	EligibleDate string        `json:"eligible_date"`
}

func settlementPreview(ctx context.Context, submissions *ent.SubmissionClient, assignments *ent.AssignmentClient, through string) (SettlementPreview, error) {
	out := SettlementPreview{
		Through:   through,
		AmountFen: "0",
		Lines:     []SettlementPreviewLine{},
	}
	if !app.ValidDate(through) {
		return out, app.Invalid("结算日期无效")
	}

	rows, err := submissions.Query().
		Where(
			submission.StatusIn(enum.SubmissionApproved, enum.SubmissionWaived),
			submission.SettlementIDEQ(""),
			submission.ScreenshotNEQ(""),
			submission.EligibleDateLTE(through),
			submission.VisibilityIn(enum.VisibilityManualVerified, enum.VisibilityVisible),
		).
		Limit(1001).
		All(ctx)
	if err != nil {
		return out, err
	}

	if len(rows) > 1000 {
		return out, app.Conflict("本批超过1000条，请缩小结算范围")
	}

	total := decimal.Zero
	for _, platformSubmission := range rows {
		assignmentRecord, err := assignments.Get(ctx, platformSubmission.AssignmentID)
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
		})
	}

	out.Count = int64(len(out.Lines))
	out.AmountFen = total.String()
	return out, nil
}

func (s *Service) PreviewSettlement(ctx context.Context, through string) (SettlementPreview, error) {
	return settlementPreview(ctx, s.Client.Submission, s.Client.Assignment, through)
}
