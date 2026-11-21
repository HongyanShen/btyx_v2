package identity

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

// Service owns the identity use cases. The runtime supplies shared infrastructure.
type Service struct {
	*app.Runtime
	WechatPhone interface {
		Number(context.Context, string) (string, error)
	}
	Store       objectstore.Gateway
	Verifier    DocumentVerifier
	IdentityOCR interface {
		Recognize(context.Context, string) (name string, number string, err error)
	}
}
