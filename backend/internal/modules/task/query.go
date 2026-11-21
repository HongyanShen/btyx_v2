package task

import (
	"context"
	"slices"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Tasks(ctx context.Context) ([]VersionDTO, error) {
	taskRecords, err := s.Client.Task.Query().
		Order(ent.Desc(task.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]VersionDTO, 0, len(taskRecords))
	if len(taskRecords) == 0 {
		return out, nil
	}

	taskIDs := make([]string, 0, len(taskRecords))
	versionIDs := make([]string, 0, len(taskRecords))
	for _, taskRecord := range taskRecords {
		taskIDs = append(taskIDs, taskRecord.ID)
		versionIDs = append(versionIDs, taskRecord.CurrentVersionID)
	}

	versions, err := s.Client.TaskVersion.Query().
		Where(taskversion.IDIn(versionIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	byVersionID := make(map[string]*ent.TaskVersion, len(versions))
	for _, version := range versions {
		byVersionID[version.ID] = version
	}

	allocations, err := s.poolAllocations(ctx, taskIDs)
	if err != nil {
		return nil, err
	}

	expiries, err := s.poolExpiries(ctx, taskIDs)
	if err != nil {
		return nil, err
	}

	mediaByFileID, err := s.poolFileMedia(ctx, versions)
	if err != nil {
		return nil, err
	}

	policy, err := LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return nil, err
	}

	var next *ent.TaskVersion
	if policy.Online {
		next, err = unallocatedVersions(s.Client, s.Day()).
			Order(ent.Asc(taskversion.FieldCreatedAt), ent.Asc(taskversion.FieldID)).
			Select(taskversion.FieldID).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, err
		}
	}

	for _, taskRecord := range taskRecords {
		version := byVersionID[taskRecord.CurrentVersionID]
		if version == nil {
			return nil, app.NotFound()
		}

		currentVersion := versionDTO(version)
		currentVersion.Online = taskRecord.Online
		currentVersion.UploadedAt = &taskRecord.CreatedAt
		currentVersion.Allocation = allocations[taskRecord.ID]
		currentVersion.LastExpiry = expiries[taskRecord.ID]
		currentVersion.NextInQueue = next != nil && next.ID == version.ID
		for _, fileID := range version.Files {
			mediaType, found := mediaByFileID[fileID]
			if found && !slices.Contains(currentVersion.MediaTypes, mediaType) {
				currentVersion.MediaTypes = append(currentVersion.MediaTypes, mediaType)
			}
		}

		out = append(out, currentVersion)
	}

	return out, nil
}

func (s *Service) poolFileMedia(ctx context.Context, versions []*ent.TaskVersion) (map[string]enum.FileMediaType, error) {
	var fileIDs []string
	for _, version := range versions {
		fileIDs = append(fileIDs, version.Files...)
	}

	mediaByFileID := make(map[string]enum.FileMediaType, len(fileIDs))
	if len(fileIDs) == 0 {
		return mediaByFileID, nil
	}

	records, err := s.Client.FileObject.Query().
		Where(fileobject.IDIn(fileIDs...)).
		Select(fileobject.FieldID, fileobject.FieldMediaType).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		mediaByFileID[record.ID] = record.MediaType
	}

	return mediaByFileID, nil
}

func (s *Service) poolAllocations(ctx context.Context, taskIDs []string) (map[string]*PoolAllocationDTO, error) {
	records, err := s.Client.Assignment.Query().
		Where(assignment.AllocationKeyIn(taskIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	allocations := make(map[string]*PoolAllocationDTO, len(records))
	if len(records) == 0 {
		return allocations, nil
	}

	assignmentIDs := make([]string, 0, len(records))
	for _, record := range records {
		assignmentIDs = append(assignmentIDs, record.ID)
		allocations[record.TaskID] = &PoolAllocationDTO{
			ID:        record.ID,
			UserID:    record.UserID,
			Source:    record.Source,
			Steps:     record.Steps,
			Settled:   record.Status == enum.AssignmentSettled,
			ExpiresAt: record.ExpiresAt,
		}
	}

	submissions, err := s.Client.Submission.Query().
		Where(submission.AssignmentIDIn(assignmentIDs...)).
		Select(submission.FieldID, submission.FieldAssignmentID, submission.FieldStatus).
		All(ctx)
	if err != nil {
		return nil, err
	}

	links := make(map[string]int, len(records))
	unreviewed := make(map[string]bool, len(records))
	for _, link := range submissions {
		links[link.AssignmentID]++
		if link.Status == enum.SubmissionPending || link.Status == enum.SubmissionRejected {
			unreviewed[link.AssignmentID] = true
		}
	}

	for _, record := range records {
		allocation := allocations[record.TaskID]
		allocation.HasLink = links[record.ID] > 0
		allocation.LinksDone = allocation.HasLink && links[record.ID] == len(record.Platforms)
		allocation.Reviewed = allocation.LinksDone && !unreviewed[record.ID]
	}

	return allocations, nil
}

func (s *Service) poolExpiries(ctx context.Context, taskIDs []string) (map[string]*PoolExpiryDTO, error) {
	rows, err := s.Client.Assignment.Query().
		Where(assignment.TaskIDIn(taskIDs...), assignment.StatusEQ(enum.AssignmentExpired)).
		Select(assignment.FieldID, assignment.FieldUserID, assignment.FieldTaskID, assignment.FieldSource, assignment.FieldExpiresAt).
		Order(ent.Desc(assignment.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*PoolExpiryDTO)
	for _, record := range rows {
		if _, found := result[record.TaskID]; found {
			continue
		}

		result[record.TaskID] = &PoolExpiryDTO{
			UserID:    record.UserID,
			Source:    record.Source,
			ExpiredAt: record.ExpiresAt,
		}
	}

	return result, nil
}

func (s *Service) Versions(ctx context.Context, taskID string) ([]VersionDTO, error) {
	versions, err := s.Client.TaskVersion.Query().
		Where(taskversion.TaskIDEQ(taskID)).
		Order(ent.Desc(taskversion.FieldNumber)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := []VersionDTO{}
	for _, version := range versions {
		out = append(out, versionDTO(version))
	}

	return out, nil
}

func (s *Service) Assignment(ctx context.Context, userID, id string) (AssignmentDTO, error) {
	return s.assignment(ctx, userID, id, enum.RoleUser)
}

func (s *Service) AdminAssignment(ctx context.Context, id string) (AssignmentDTO, error) {
	return s.assignment(ctx, "", id, enum.RoleAdmin)
}

func (s *Service) assignment(ctx context.Context, userID, id string, role enum.Role) (AssignmentDTO, error) {
	assignmentRecord, err := s.Client.Assignment.Get(ctx, id)
	if err != nil {
		return AssignmentDTO{}, app.WrapMissing(err)
	}

	if role != enum.RoleAdmin && userID != assignmentRecord.UserID {
		return AssignmentDTO{}, app.NotFound()
	}

	version, err := s.Client.TaskVersion.Get(ctx, assignmentRecord.VersionID)
	if err != nil {
		return AssignmentDTO{}, err
	}

	submissions, err := s.Client.Submission.Query().
		Where(submission.AssignmentIDEQ(id)).
		All(ctx)
	if err != nil {
		return AssignmentDTO{}, err
	}

	return assignmentDetail(assignmentRecord, version, submissions), nil
}

func assignmentDetail(assignmentRecord *ent.Assignment, version *ent.TaskVersion, submissions []*ent.Submission) AssignmentDTO {
	detail := AssignmentDTO{
		ID:          assignmentRecord.ID,
		UserID:      assignmentRecord.UserID,
		TaskID:      assignmentRecord.TaskID,
		VersionID:   assignmentRecord.VersionID,
		PriceFen:    assignmentRecord.PriceFen.String(),
		Title:       version.Title,
		Body:        version.Body,
		Files:       version.Files,
		Platforms:   assignmentRecord.Platforms,
		Status:      assignmentRecord.Status,
		Source:      assignmentRecord.Source,
		SourceID:    assignmentRecord.SourceID,
		Date:        assignmentRecord.BusinessDate,
		DueDate:     assignmentRecord.DueDate,
		ExpiresAt:   assignmentRecord.ExpiresAt,
		Submissions: []SubmissionDTO{},
		Steps:       []StepDTO{},
	}
	for _, record := range submissions {
		detail.Submissions = append(detail.Submissions, SubmissionDTO{
			ID:         record.ID,
			Platform:   record.Platform,
			URL:        record.URL,
			Screenshot: record.Screenshot,
			Status:     record.Status,
			Reason:     record.Reason,
			Revision:   record.Revision,
			Settled:    record.SettlementID != "",
			Visibility: record.Visibility,
		})
	}

	for _, step := range []enum.TaskStep{
		enum.StepContent,
		enum.StepPublish,
		enum.StepLinks,
		enum.StepData,
		enum.StepSettle,
	} {
		done := slices.Contains(assignmentRecord.Steps, step)
		if step == enum.StepLinks {
			done = len(submissions) == len(assignmentRecord.Platforms)
		}

		if step == enum.StepData {
			done = len(submissions) == len(assignmentRecord.Platforms)
			for _, record := range submissions {
				done = done && record.Screenshot != ""
			}
		}

		if step == enum.StepSettle {
			done = assignmentRecord.Status == enum.AssignmentSettled
		}

		detail.Steps = append(detail.Steps, StepDTO{
			Key:   step,
			Label: step.Label(),
			Done:  done,
		})
	}

	return detail
}

func (s *Service) Assignments(ctx context.Context, userID string, role enum.Role) ([]AssignmentDTO, error) {
	query := s.Client.Assignment.Query()
	if role != enum.RoleAdmin {
		query = query.Where(assignment.UserIDEQ(userID))
	}

	rows, err := query.Order(ent.Desc(assignment.FieldCreatedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := []AssignmentDTO{}
	for _, record := range rows {
		detail, err := s.assignment(ctx, userID, record.ID, role)
		if err != nil {
			return nil, err
		}

		out = append(out, detail)
	}

	return out, nil
}
