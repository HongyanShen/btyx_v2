package files

import (
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

const MaximumSize int64 = 25 << 20

type InitiateInput struct {
	Name      string             `json:"name"`
	MediaType enum.FileMediaType `json:"media_type"`
	Size      int64              `json:"size"`
}

type UploadTicket struct {
	ID        string                   `json:"id"`
	URL       string                   `json:"url"`
	Fields    objectstore.UploadFields `json:"fields"`
	ExpiresAt *time.Time               `json:"expires_at"`
	Completed bool                     `json:"completed"`
}

type DownloadDTO struct {
	Storage   enum.FileStorage   `json:"storage"`
	Name      string             `json:"name"`
	MediaType enum.FileMediaType `json:"media_type"`
	URL       string             `json:"url"`
	ExpiresAt *time.Time         `json:"expires_at"`
}

func toDTO(record *ent.FileObject) DTO {
	return DTO{
		ID:        record.ID,
		Name:      record.Name,
		MediaType: record.MediaType,
		Size:      record.Size,
	}
}

func unavailable(message string) error {
	return &app.Error{
		Status:  503,
		Code:    enum.ErrorUnavailable,
		Message: message,
	}
}
