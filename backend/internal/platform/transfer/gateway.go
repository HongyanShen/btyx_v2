package transfer

import (
	"context"
	"errors"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

var ErrNotFound = errors.New("original merchant transfer bill not found")

type Request struct {
	ID        string
	OpenID    string
	AmountFen string
}

type Result struct {
	Status      enum.WithdrawalStatus
	ProviderID  string
	PackageInfo string
	AmountFen   string
	OpenID      string
}

// Gateway is the third-party boundary. Retried requests must query the
// existing merchant transfer ID instead of creating a second transfer.
type Gateway interface {
	Submit(context.Context, Request) (Result, error)
	Query(context.Context, string) (Result, error)
}
