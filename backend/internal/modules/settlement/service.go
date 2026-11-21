package settlement

import (
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

// Service owns the settlement use cases. The runtime supplies shared infrastructure.
type Service struct {
	*app.Runtime
}
