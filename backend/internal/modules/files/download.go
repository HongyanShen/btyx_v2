package files

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

func (s *Service) Download(ctx context.Context, userID string, role enum.Role, id string) (DownloadDTO, error) {
	record, err := s.FileForRead(ctx, userID, role, id)
	if err != nil {
		return DownloadDTO{}, err
	}

	ticket := DownloadDTO{
		Storage:   record.Storage,
		Name:      record.Name,
		MediaType: record.MediaType,
	}
	if record.Storage == enum.FileStorageLocal {
		return ticket, nil
	}

	if s.Store == nil {
		return DownloadDTO{}, unavailable("TOS 文件存储尚未配置")
	}

	const lifetime = 5 * time.Minute
	url, err := s.Store.SignDownload(ctx, objectstore.Reference{
		Bucket:    record.Bucket,
		Key:       record.ObjectKey,
		VersionID: record.VersionID,
	}, lifetime)
	if err != nil {
		return DownloadDTO{}, unavailable("暂时无法签发下载地址，请重试")
	}

	expiresAt := s.Now().Add(lifetime)

	ticket.URL = url
	ticket.ExpiresAt = &expiresAt
	return ticket, nil
}
