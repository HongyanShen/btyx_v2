package creative

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"time"
)

type Tier struct {
	Likes     int    `json:"likes"`
	RewardFen string `json:"reward_fen"`
}

type TaskInput struct {
	Title         string             `json:"title"`
	Requirements  string             `json:"requirements"`
	CopyText      string             `json:"copy_text"`
	Topics        string             `json:"topics"`
	Files         []string           `json:"files"`
	Platforms     []enum.Platform    `json:"platforms"`
	MediaType     enum.FileMediaType `json:"media_type"`
	CreationFen   string             `json:"creation_fen"`
	CombinedLikes bool               `json:"combined_likes"`
	PublishDays   int                `json:"publish_days"`
	StartsAt      time.Time          `json:"starts_at"`
	EndsAt        time.Time          `json:"ends_at"`
	RewardAt      time.Time          `json:"reward_at"`
	Tiers         []Tier             `json:"tiers"`
}

type TaskDTO struct {
	TaskInput
	TaskCode   string `json:"task_code"`
	ID         string `json:"id"`
	Online     bool   `json:"online"`
	PublishFen string `json:"publish_fen"`
}

type Link struct {
	Reason     string        `json:"reason,omitempty"`
	Platform   enum.Platform `json:"platform"`
	URL        string        `json:"url"`
	Screenshot string        `json:"screenshot"`
	Likes      *int          `json:"likes"`
}

type Earning struct {
	LikesReward bool      `json:"likes_reward"`
	AmountFen   string    `json:"amount_fen"`
	EligibleAt  time.Time `json:"eligible_at"`
	BatchID     string    `json:"batch_id"`
}

type WorkDTO struct {
	ID           string                  `json:"id"`
	UserID       string                  `json:"user_id"`
	Task         TaskDTO                 `json:"task"`
	Status       enum.CreativeWorkStatus `json:"status"`
	Revision     int                     `json:"revision"`
	CopyText     string                  `json:"copy_text"`
	Files        []string                `json:"files"`
	Reason       string                  `json:"reason"`
	Links        []Link                  `json:"links"`
	Earnings     []Earning               `json:"earnings"`
	BusinessDate string                  `json:"business_date"`
}

type SubmitInput struct {
	Revision int      `json:"revision"`
	CopyText string   `json:"copy_text"`
	Files    []string `json:"files"`
}

type ReviewInput struct {
	Revision int    `json:"revision"`
	Approve  bool   `json:"approve"`
	Reason   string `json:"reason"`
}

type LinksInput struct {
	Links []Link `json:"links"`
}

type Catalog struct {
	Tasks  []TaskDTO `json:"tasks"`
	Mine   []WorkDTO `json:"mine"`
	Online bool      `json:"online"`
	Limit  int       `json:"limit"`
	Used   int       `json:"used"`
}
