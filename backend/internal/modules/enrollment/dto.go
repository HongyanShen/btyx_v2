package enrollment

import (
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
)

type ApplyInput struct {
	identity.DocumentInput
	Phone                string   `json:"phone"`
	Douyin               string   `json:"douyin"`
	XHS                  string   `json:"xhs"`
	AgreementID          string   `json:"agreement_id"`
	PreviousEnrollmentID string   `json:"previous_enrollment_id,omitempty"`
	Tags                 []string `json:"-"`
}

type ReviewInput struct {
	EnrollmentID string             `json:"enrollment_id,omitempty"`
	Approved     bool               `json:"approved"`
	Reason       string             `json:"reason"`
	Tags         []string           `json:"tags"`
	UpdatedAt    *time.Time         `json:"enrollment_updated_at"`
	RecentPost   *bool              `json:"recent_post"`
	NewAccount   *bool              `json:"new_account"`
	AgeGroup     enum.EnrollmentAge `json:"age_group"`
	PortraitOnly bool               `json:"portrait_only"`
	TagsOnly     bool               `json:"tags_only"`
}

type MarketingTagDTO struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}
