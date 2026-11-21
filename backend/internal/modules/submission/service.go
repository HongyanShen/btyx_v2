package submission

import (
	tasks "baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

// Service owns the submission use cases. The runtime supplies shared infrastructure.
type Service struct {
	*app.Runtime
	TaskQueries *tasks.Service
}
