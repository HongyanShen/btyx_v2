package identity

import (
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
)

type LoginResult struct {
	Token  string    `json:"token"`
	UserID string    `json:"user_id"`
	Role   enum.Role `json:"role"`
}

type Principal struct {
	ID       string
	Role     enum.Role
	Operator OperatorDTO
}

type OperatorDTO struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	SuperAdmin   bool     `json:"super_admin"`
	Disabled     bool     `json:"disabled"`
	AllPages     bool     `json:"all_pages"`
	AllEndpoints bool     `json:"all_endpoints"`
	Pages        []string `json:"pages"`
	Endpoints    []string `json:"endpoints"`
}

type OperatorGrants struct {
	AllPages     bool     `json:"all_pages,omitempty"`
	AllEndpoints bool     `json:"all_endpoints,omitempty"`
	Pages        []string `json:"pages"`
	Endpoints    []string `json:"endpoints"`
}

type CreateOperatorInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	OperatorGrants
}

type UpdateOperatorInput struct {
	Password string `json:"password"`
	Disabled *bool  `json:"disabled"`
	OperatorGrants
}

type OperatorPage struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

type OperatorEndpoint struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type OperatorPermissionCatalog struct {
	Pages     []OperatorPage     `json:"pages"`
	Endpoints []OperatorEndpoint `json:"endpoints"`
}

type UserDTO struct {
	ID                           string                     `json:"id"`
	Name                         string                     `json:"name"`
	Phone                        string                     `json:"phone"`
	Nickname                     string                     `json:"nickname"`
	AvatarFileID                 string                     `json:"avatar_file_id"`
	VerifiedPhone                string                     `json:"verified_phone"`
	ContactPhone                 string                     `json:"contact_phone"`
	Wechat                       string                     `json:"wechat"`
	IdentityVerified             bool                       `json:"identity_verified"`
	IdentityDocument             *VerifiedIdentityDTO       `json:"identity_document"`
	IdentityResubmissionRequired bool                       `json:"identity_resubmission_required"`
	EnrollmentID                 string                     `json:"enrollment_id,omitempty"`
	EnrollmentDocument           *VerifiedIdentityDTO       `json:"enrollment_document,omitempty"`
	EnrollmentUpdatedAt          *time.Time                 `json:"enrollment_updated_at"`
	EnrollmentSubmittedAt        *time.Time                 `json:"enrollment_submitted_at,omitempty"`
	EnrollmentHasPhotos          bool                       `json:"enrollment_has_photos,omitempty"`
	EnrollmentPhotoPreviews      *EnrollmentPhotoPreviewDTO `json:"enrollment_photo_previews,omitempty"`
	EnrollmentRecentPost         *bool                      `json:"enrollment_recent_post,omitempty"`
	EnrollmentNewAccount         *bool                      `json:"enrollment_new_account,omitempty"`
	EnrollmentAgeGroup           *enum.EnrollmentAge        `json:"enrollment_age_group,omitempty"`
	Douyin                       string                     `json:"douyin"`
	XHS                          string                     `json:"xhs"`
	Status                       enum.EnrollmentStatus      `json:"status"`
	Reason                       string                     `json:"reason"`
	Tags                         []string                   `json:"tags"`
	Blocked                      bool                       `json:"blocked"`
	BlockReason                  string                     `json:"block_reason,omitempty"`
	BlockInfo                    *DistributorBlockDTO       `json:"block_info,omitempty"`
	DistributorDetails           *DistributorDetailsDTO     `json:"distributor_details,omitempty"`
	Consented                    bool                       `json:"consented"`
}

type DistributorBlockDTO struct {
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
	Operator string    `json:"operator,omitempty"`
}

type DistributorDetailsDTO struct {
	Phone      string                    `json:"phone"`
	Identity   string                    `json:"identity"`
	UpdatedAt  time.Time                 `json:"updated_at"`
	TodayTasks []task.DistributorTaskDTO `json:"today_tasks"`
}

type EnrollmentPhotoPreviewDTO struct {
	FrontURL    string `json:"front_url"`
	BackURL     string `json:"back_url"`
	Unavailable bool   `json:"unavailable"`
}

type VerifiedIdentityDTO struct {
	Number      string `json:"number"`
	FrontFileID string `json:"front_file_id"`
	BackFileID  string `json:"back_file_id"`
}

type UpdateProfileInput struct {
	Nickname     *string `json:"nickname"`
	AvatarFileID *string `json:"avatar_file_id"`
}

type VerifiedPhoneDTO struct {
	VerifiedPhone       string `json:"verified_phone"`
	ContactPhoneReceipt string `json:"contact_phone_receipt,omitempty"`
}
