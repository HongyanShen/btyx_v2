package task

import (
	"context"
	"math"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
)

const reviewPageSize = 12

func (s *Service) ReviewPage(ctx context.Context, page int, status enum.AssignmentStatus, platform enum.Platform) ([]AssignmentDTO, int, error) {
	if page < 1 || page > math.MaxInt/reviewPageSize {
		return nil, 0, app.Invalid("分页参数无效")
	}

	if status != "" && !status.Valid() {
		return nil, 0, app.Invalid("任务状态无效")
	}

	if platform != "" && !platform.Valid() {
		return nil, 0, app.Invalid("发布平台无效")
	}

	query := s.Client.Assignment.Query()
	var submissionStatus enum.SubmissionStatus
	switch status {
	case enum.AssignmentPendingReview:
		submissionStatus = enum.SubmissionPending
	case enum.AssignmentRejected:
		submissionStatus = enum.SubmissionRejected
	case "":
	default:
		query.Where(assignment.StatusEQ(status))
	}

	if platform != "" || submissionStatus != "" {
		query.Where(func(selector *sql.Selector) {
			submissions := sql.Table(submission.Table)
			matching := sql.Select(submissions.C(submission.FieldAssignmentID)).
				From(submissions)
			if platform != "" {
				matching.Where(sql.EQ(submissions.C(submission.FieldPlatform), platform))
			}

			if submissionStatus != "" {
				matching.Where(sql.EQ(submissions.C(submission.FieldStatus), submissionStatus))
			}

			selector.Where(sql.In(selector.C(assignment.FieldID), matching))
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * reviewPageSize
	if offset >= total {
		return []AssignmentDTO{}, total, nil
	}

	rows, err := query.
		Order(ent.Desc(assignment.FieldCreatedAt, assignment.FieldID)).
		Offset(offset).
		Limit(reviewPageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	details, err := s.reviewDetails(ctx, rows)
	return details, total, err
}

func (s *Service) reviewDetails(ctx context.Context, rows []*ent.Assignment) ([]AssignmentDTO, error) {
	details := make([]AssignmentDTO, 0, len(rows))
	if len(rows) == 0 {
		return details, nil
	}

	assignmentIDs := make([]string, 0, len(rows))
	versionIDs := make([]string, 0, len(rows))
	taskIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		taskIDs = append(taskIDs, row.TaskID)
		assignmentIDs = append(assignmentIDs, row.ID)
		versionIDs = append(versionIDs, row.VersionID)
	}

	codes, err := taskCodes(ctx, s.Client, taskIDs)
	if err != nil {
		return nil, err
	}

	versions, err := s.Client.TaskVersion.Query().
		Where(taskversion.IDIn(versionIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	submissions, err := s.Client.Submission.Query().
		Where(submission.AssignmentIDIn(assignmentIDs...)).
		Order(ent.Asc(submission.FieldPlatform)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	events, err := s.Client.AuditEvent.Query().
		Where(
			auditevent.ReferenceIDIn(assignmentIDs...),
			auditevent.ActionEQ(enum.AuditSubmissionSubmit),
		).
		Order(ent.Asc(auditevent.FieldCreatedAt)).
		Select(auditevent.FieldReferenceID, auditevent.FieldReason, auditevent.FieldCreatedAt).
		All(ctx)
	if err != nil {
		return nil, err
	}

	versionsByID := make(map[string]*ent.TaskVersion, len(versions))
	for _, version := range versions {
		versionsByID[version.ID] = version
	}

	submissionsByAssignment := make(map[string][]*ent.Submission, len(rows))
	for _, record := range submissions {
		submissionsByAssignment[record.AssignmentID] = append(submissionsByAssignment[record.AssignmentID], record)
	}

	type submissionKey struct {
		assignmentID string
		platform     enum.Platform
	}

	submissionTimes := make(map[submissionKey]time.Time, len(events))
	for _, event := range events {
		key := submissionKey{
			assignmentID: event.ReferenceID,
			platform:     enum.Platform(event.Reason),
		}
		submissionTimes[key] = event.CreatedAt
	}

	for _, row := range rows {
		version := versionsByID[row.VersionID]
		if version == nil {
			return nil, app.NotFound()
		}

		records := submissionsByAssignment[row.ID]
		detail := assignmentDetail(row, version, records)
		detail.TaskCode = codes[row.TaskID]
		for index, record := range records {
			key := submissionKey{
				assignmentID: row.ID,
				platform:     record.Platform,
			}
			if submittedAt, found := submissionTimes[key]; found {
				detail.Submissions[index].SubmittedAt = &submittedAt
			} else if record.Revision == 1 {
				detail.Submissions[index].SubmittedAt = &record.CreatedAt
			}
		}

		details = append(details, detail)
	}

	return details, nil
}
