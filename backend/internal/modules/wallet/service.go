package wallet

import (
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

// Service owns the wallet use cases. The runtime supplies shared infrastructure.
type Service struct {
	*app.Runtime
}
