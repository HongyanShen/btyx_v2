package submission

import "baituber.cn/yanxuan/v2/internal/domain/enum"

type SubmitInput struct {
	Platform   enum.Platform `json:"platform"`
	URL        string        `json:"url"`
	Screenshot string        `json:"screenshot"`
}

type ReviewInput struct {
	Platform enum.Platform `json:"platform"`
	Revision int64         `json:"revision"`
	Approved bool          `json:"approved"`
	Reason   string        `json:"reason"`
}

type ReviewRevision struct {
	Platform enum.Platform `json:"platform"`
	Revision int64         `json:"revision"`
}
