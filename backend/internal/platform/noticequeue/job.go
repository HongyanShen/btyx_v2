package noticequeue

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
)

type Args struct {
	ID          string `json:"notification_id"`
	RequestID   string `json:"request_id"`
	TraceParent string `json:"traceparent,omitempty"`
}

func (Args) Kind() string {
	return "wechat_subscription"
}

func NewArgs(ctx context.Context, id string) Args {

	return Args{
		ID:          id,
		RequestID:   logging.RequestID(ctx),
		TraceParent: telemetry.Parent(ctx),
	}
}
