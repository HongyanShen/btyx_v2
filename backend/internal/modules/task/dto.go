package task

import (
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

type TaskInput struct {
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Project   string          `json:"project"`
	ContentID string          `json:"content_id"`
	Files     []string        `json:"files"`
	PriceFen  string          `json:"price_fen"`
	Platforms []enum.Platform `json:"platforms"`
	Tags      []string        `json:"tags"`
	Deadline  string          `json:"deadline"`
}

type VersionDTO struct {
	ID          string               `json:"id"`
	TaskID      string               `json:"task_id"`
	Number      int64                `json:"number"`
	CreatorID   string               `json:"creator_id"`
	CreatedAt   time.Time            `json:"created_at"`
	UploadedAt  *time.Time           `json:"uploaded_at,omitempty"`
	MediaTypes  []enum.FileMediaType `json:"media_types,omitempty"`
	Online      bool                 `json:"online"`
	Allocation  *PoolAllocationDTO   `json:"allocation,omitempty"`
	LastExpiry  *PoolExpiryDTO       `json:"last_expiry,omitempty"`
	NextInQueue bool                 `json:"next_in_queue,omitempty"`
	TaskInput
}

type PoolAllocationDTO struct {
	ID        string                `json:"id"`
	UserID    string                `json:"user_id"`
	Source    enum.AssignmentSource `json:"source"`
	HasLink   bool                  `json:"has_link"`
	Steps     []enum.TaskStep       `json:"steps,omitempty"`
	LinksDone bool                  `json:"links_done"`
	Reviewed  bool                  `json:"reviewed"`
	Settled   bool                  `json:"settled"`
	ExpiresAt *time.Time            `json:"expires_at,omitempty"`
}

type PoolExpiryDTO struct {
	UserID    string                `json:"user_id"`
	Source    enum.AssignmentSource `json:"source"`
	ExpiredAt *time.Time            `json:"expired_at,omitempty"`
}

type TodayAvailabilityDTO struct {
	HasContent bool   `json:"has_content"`
	PriceFen   string `json:"price_fen"`
}

type StepDTO struct {
	Key   enum.TaskStep `json:"key"`
	Label string        `json:"label"`
	Done  bool          `json:"done"`
}

type DistributorTaskDTO struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Status      enum.AssignmentStatus `json:"status"`
	Source      enum.AssignmentSource `json:"source"`
	Unpublished bool                  `json:"unpublished"`
}

type AssignmentDTO struct {
	ID          string                `json:"id"`
	UserID      string                `json:"user_id"`
	TaskID      string                `json:"task_id"`
	VersionID   string                `json:"version_id"`
	PriceFen    string                `json:"price_fen"`
	Title       string                `json:"title"`
	Body        string                `json:"body"`
	Files       []string              `json:"files"`
	Platforms   []enum.Platform       `json:"platforms"`
	Status      enum.AssignmentStatus `json:"status"`
	Source      enum.AssignmentSource `json:"source"`
	SourceID    string                `json:"source_id"`
	Date        string                `json:"date"`
	DueDate     string                `json:"due_date"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
	Steps       []StepDTO             `json:"steps"`
	Submissions []SubmissionDTO       `json:"submissions"`
}

type SubmissionDTO struct {
	ID          string                `json:"id"`
	Platform    enum.Platform         `json:"platform"`
	URL         string                `json:"url"`
	Screenshot  string                `json:"screenshot"`
	Status      enum.SubmissionStatus `json:"status"`
	Reason      string                `json:"reason"`
	Revision    int64                 `json:"revision"`
	Settled     bool                  `json:"settled"`
	Visibility  enum.Visibility       `json:"visibility"`
	SubmittedAt *time.Time            `json:"submitted_at,omitempty"`
}
