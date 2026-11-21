package enrollment

import (
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

// Service owns the enrollment use cases. The runtime supplies shared infrastructure.
type Service struct {
	*app.Runtime
	Verifier identity.DocumentVerifier
}
