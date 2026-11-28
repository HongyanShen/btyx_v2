package settlement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/ent/predicate"
	"baituber.cn/yanxuan/v2/ent/settlementbatch"
	"baituber.cn/yanxuan/v2/ent/settlementline"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/ent/withdrawal"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/shopspring/decimal"
)

type MonthDTO struct {
	Today        string             `json:"today"`
	WithdrawnFen string             `json:"withdrawn_fen"`
	Withdrawals  int                `json:"withdrawals"`
	Weeks        []WeekDTO          `json:"weeks"`
	Distributors []identity.UserDTO `json:"distributors"`
}

type WeekDTO struct {
	Through     string        `json:"through"`
	Start       string        `json:"start"`
	End         string        `json:"end"`
	ReviewToken string        `json:"review_token"`
	Weekly      bool          `json:"weekly"`
	Due         bool          `json:"due"`
	Lines       []WeekLineDTO `json:"lines"`
}

type WeekLineDTO struct {
	Creative      bool                  `json:"creative,omitempty"`
	ID            string                `json:"id"`
	AssignmentID  string                `json:"assignment_id"`
	UserID        string                `json:"user_id"`
	Project       string                `json:"project"`
	Title         string                `json:"title"`
	PublishedOn   string                `json:"published_on"`
	TaskDate      string                `json:"task_date"`
	Platform      enum.Platform         `json:"platform"`
	URL           string                `json:"url"`
	Screenshot    string                `json:"screenshot"`
	HasScreenshot bool                  `json:"has_screenshot"`
	Status        enum.SubmissionStatus `json:"status"`
	Visibility    enum.Visibility       `json:"visibility"`
	AmountFen     string                `json:"amount_fen"`
	Due           bool                  `json:"due"`
	EligibleAt    *time.Time            `json:"eligible_at,omitempty"`
	CreditedAt    *time.Time            `json:"credited_at"`
	CreditedBy    string                `json:"credited_by"`
}

type MonthAccess struct {
	Pending     bool
	History     bool
	Withdrawals bool
	Screenshots bool
}

func (s *Service) Month(ctx context.Context, month string, access MonthAccess) (MonthDTO, error) {
	now := app.BeijingTime(s.Now())
	start, err := time.ParseInLocation("2006-01", month, now.Location())
	if err != nil || start.Format("2006-01") != month || start.Year() < 1 {
		return MonthDTO{}, app.Invalid("结算月份无效")
	}

	end := start.AddDate(0, 1, 0)
	report := MonthDTO{
		Today:        now.Format(time.DateOnly),
		WithdrawnFen: "0",
		Weeks:        []WeekDTO{},
		Distributors: []identity.UserDTO{},
	}
	if access.Withdrawals {
		rows, err := s.Client.Withdrawal.Query().
			Where(
				withdrawal.StatusEQ(enum.WithdrawalSuccess),
				withdrawal.UpdatedAtGTE(start),
				withdrawal.UpdatedAtLT(end),
			).
			All(ctx)
		if err != nil {
			return MonthDTO{}, err
		}

		total := decimal.Zero
		for _, row := range rows {
			total = total.Add(row.AmountFen)
		}

		report.WithdrawnFen = total.String()
		report.Withdrawals = len(rows)
	}

	policy, err := task.LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return MonthDTO{}, err
	}

	periods := make(map[string]*WeekDTO)
	if policy.Schedule.SettlementFrequency == enum.SettlementWeekly || policy.Schedule.SettlementFrequency == enum.SettlementMonthly {
		for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
			if _, _, _, scheduled := policy.Schedule.CalendarPeriod(day); scheduled {
				date := day.Format(time.DateOnly)
				periods[date] = newMonthPeriod(policy.Schedule, date, now, false)
			}
		}
	}

	if err := s.loadMonthLines(ctx, start.Format(time.DateOnly), end.Format(time.DateOnly), periods, policy.Schedule, now, access); err != nil {
		return MonthDTO{}, err
	}

	for _, period := range periods {
		report.Weeks = append(report.Weeks, *period)
	}

	slices.SortFunc(report.Weeks, func(a, b WeekDTO) int { return strings.Compare(b.Through, a.Through) })
	return report, nil
}

func newMonthPeriod(schedule task.MarketingSchedule, date string, now time.Time, legacy bool) *WeekDTO {
	day, _ := time.ParseInLocation(time.DateOnly, date, now.Location())
	start, end, opensAt, scheduled := schedule.CalendarPeriod(day)
	weekly := scheduled && schedule.SettlementFrequency == enum.SettlementWeekly
	if !scheduled && legacy && day.Weekday() == time.Monday {
		start = day.AddDate(0, 0, -7)
		end = day.AddDate(0, 0, -1)
		weekly = true
	}

	return &WeekDTO{
		Through: date,
		Start:   start.Format(time.DateOnly),
		End:     end.Format(time.DateOnly),
		Weekly:  weekly,
		Due:     scheduled && !now.Before(opensAt),
		Lines:   []WeekLineDTO{},
	}
}

func (s *Service) loadMonthLines(ctx context.Context, start, end string, periods map[string]*WeekDTO, schedule task.MarketingSchedule, now time.Time, access MonthAccess) error {
	filters := make([]predicate.Submission, 0, 2)
	if access.Pending {
		filters = append(filters, submission.And(
			submission.EligibleDateGTE(start),
			submission.EligibleDateLT(end),
			submission.SettlementIDEQ(""),
			submission.StatusIn(enum.SubmissionApproved, enum.SubmissionWaived, enum.SubmissionInvalid),
		))
	}

	batches := make(map[string]*ent.SettlementBatch)
	posted := make(map[string]*ent.SettlementLine)
	operators := make(map[string]string)
	if access.History {
		rows, err := s.Client.SettlementBatch.Query().
			Where(settlementbatch.ThroughGTE(start), settlementbatch.ThroughLT(end)).
			All(ctx)
		if err != nil {
			return err
		}

		batchIDs := make([]string, 0, len(rows))
		operatorIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			batches[row.ID] = row
			batchIDs = append(batchIDs, row.ID)
			operatorIDs = append(operatorIDs, row.CreatorID)
		}

		if len(batchIDs) > 0 {
			lines, err := s.Client.SettlementLine.Query().
				Where(settlementline.BatchIDIn(batchIDs...)).
				All(ctx)
			if err != nil {
				return err
			}

			for _, line := range lines {
				posted[line.SubmissionID] = line
			}

			filters = append(filters, submission.SettlementIDIn(batchIDs...))
			accounts, err := s.Client.Operator.Query().Where(operator.IDIn(operatorIDs...)).All(ctx)
			if err != nil {
				return err
			}

			for _, account := range accounts {
				operators[account.ID] = account.Username
			}
		}
	}

	if len(filters) == 0 {
		return nil
	}

	rows, err := s.Client.Submission.Query().
		Where(submission.Or(filters...)).
		Order(ent.Asc(submission.FieldCreatedAt), ent.Asc(submission.FieldID)).
		All(ctx)
	if err != nil {
		return err
	}

	assignments, versions, err := s.monthTasks(ctx, rows)
	if err != nil {
		return err
	}

	revisions := make(map[string][]string)
	for _, row := range rows {
		record := assignments[row.AssignmentID]
		if record == nil {
			return app.Conflict("结算记录关联的派发不存在")
		}

		version := versions[record.VersionID]
		if version == nil {
			return app.Conflict("结算记录关联的任务版本不存在")
		}

		line := WeekLineDTO{
			ID:           row.ID,
			AssignmentID: record.ID,
			UserID:       record.UserID,
			Project:      version.Project,
			Title:        version.Title,
			PublishedOn:  app.BusinessDay(row.CreatedAt),
			TaskDate:     record.BusinessDate,
			Platform:     row.Platform,
			URL:          row.URL,
			Status:       row.Status,
			Visibility:   row.Visibility,
			AmountFen:    "0",
			EligibleAt:   row.SettlementEligibleAt,
		}
		if access.Screenshots {
			line.Screenshot = row.Screenshot
		}

		date := row.EligibleDate
		line.Due = date <= now.Format(time.DateOnly) &&
			(row.SettlementEligibleAt == nil || !now.Before(*row.SettlementEligibleAt))
		if paid := posted[row.ID]; paid != nil {
			batch := batches[paid.BatchID]
			date = batch.Through
			line.AmountFen = paid.AmountFen.String()
			line.CreditedAt = &batch.CreatedAt
			line.CreditedBy = operators[batch.CreatorID]
			line.Due = true
		} else if row.SettlementID != "" {
			return app.Conflict("结算批次缺少已入账明细")
		} else if row.Status != enum.SubmissionInvalid && row.Screenshot != "" &&
			(row.Visibility == enum.VisibilityVisible || row.Visibility == enum.VisibilityManualVerified) {
			line.AmountFen = record.PriceFen.String()
			if line.Due {
				revisions[date] = append(revisions[date], settlementRevision(row, record))
			}
		}

		line.HasScreenshot = row.Screenshot != ""
		period := periods[date]
		if period == nil {
			period = newMonthPeriod(schedule, date, now, row.SettlementEligibleAt == nil)
			periods[date] = period
		}

		period.Due = period.Due || line.Due
		period.Lines = append(period.Lines, line)
	}

	if err := s.monthCreative(ctx, start, end, periods, schedule, now, access, revisions, batches, operators); err != nil {
		return err
	}

	for date, rows := range revisions {
		periods[date].ReviewToken = reviewToken(rows)
	}

	return nil
}

func (s *Service) monthTasks(ctx context.Context, rows []*ent.Submission) (map[string]*ent.Assignment, map[string]*ent.TaskVersion, error) {
	assignmentIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		assignmentIDs = append(assignmentIDs, row.AssignmentID)
	}

	records, err := s.Client.Assignment.Query().Where(assignment.IDIn(assignmentIDs...)).All(ctx)
	if err != nil {
		return nil, nil, err
	}

	assignments := make(map[string]*ent.Assignment, len(records))
	versionIDs := make([]string, 0, len(records))
	for _, record := range records {
		assignments[record.ID] = record
		versionIDs = append(versionIDs, record.VersionID)
	}

	snapshots, err := s.Client.TaskVersion.Query().Where(taskversion.IDIn(versionIDs...)).All(ctx)
	if err != nil {
		return nil, nil, err
	}

	versions := make(map[string]*ent.TaskVersion, len(snapshots))
	for _, version := range snapshots {
		versions[version.ID] = version
	}

	return assignments, versions, nil
}

func settlementRevision(row *ent.Submission, record *ent.Assignment) string {
	return row.ID + ":" + strconv.FormatInt(row.Revision, 10) + ":" + row.UpdatedAt.UTC().Format(time.RFC3339Nano) + ":" + record.PriceFen.String()
}

func reviewToken(revisions []string) string {
	slices.Sort(revisions)
	sum := sha256.Sum256([]byte(strings.Join(revisions, "\n")))

	return hex.EncodeToString(sum[:])
}

func previewToken(preview SettlementPreview) string {
	revisions := make([]string, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		revisions = append(revisions, line.revision)
	}

	return reviewToken(revisions)
}
