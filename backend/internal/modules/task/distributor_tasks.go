package task

import (
	"context"
	"slices"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func (s *Service) TodayDistributorTasks(ctx context.Context, userID string) ([]DistributorTaskDTO, error) {
	records, err := s.Client.Assignment.Query().
		Where(
			assignment.UserIDEQ(userID),
			assignment.BusinessDateEQ(s.Day()),
			assignment.StatusNEQ(enum.AssignmentRevoked),
		).
		Order(ent.Desc(assignment.FieldCreatedAt, assignment.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DistributorTaskDTO, 0, len(records))
	if len(records) == 0 {
		return result, nil
	}

	versionIDs := make([]string, 0, len(records))
	assignmentIDs := make([]string, 0, len(records))
	for _, record := range records {
		versionIDs = append(versionIDs, record.VersionID)
		assignmentIDs = append(assignmentIDs, record.ID)
	}

	versions, err := s.Client.TaskVersion.Query().
		Where(taskversion.IDIn(versionIDs...)).
		Select(taskversion.FieldID, taskversion.FieldTitle).
		All(ctx)
	if err != nil {
		return nil, err
	}

	titles := make(map[string]string, len(versions))
	for _, version := range versions {
		titles[version.ID] = version.Title
	}

	links, err := s.Client.Submission.Query().
		Where(
			submission.AssignmentIDIn(assignmentIDs...),
			submission.URLNEQ(""),
		).
		Select(submission.FieldAssignmentID, submission.FieldPlatform).
		All(ctx)
	if err != nil {
		return nil, err
	}

	published := make(map[string][]enum.Platform, len(records))
	for _, link := range links {
		published[link.AssignmentID] = append(published[link.AssignmentID], link.Platform)
	}

	for _, record := range records {
		result = append(result, DistributorTaskDTO{
			ID:          record.ID,
			Title:       titles[record.VersionID],
			Status:      record.Status,
			Source:      record.Source,
			Unpublished: hasUnpublishedPlatforms(record, published[record.ID]),
		})
	}

	return result, nil
}

func hasUnpublishedPlatforms(record *ent.Assignment, published []enum.Platform) bool {
	switch record.Status {
	case enum.AssignmentExpired, enum.AssignmentSettled, enum.AssignmentNotPayable:
		return false
	}

	if slices.Contains(record.Steps, enum.StepPublish) {
		return false
	}

	for _, platform := range record.Platforms {
		if !slices.Contains(published, platform) {
			return true
		}
	}

	return false
}
