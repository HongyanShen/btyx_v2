package files

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type Service struct {
	*app.Runtime
	Store objectstore.Gateway
}

type DTO struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	MediaType enum.FileMediaType `json:"media_type"`
	Size      int64              `json:"size"`
}
