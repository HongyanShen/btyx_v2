package files

import (
	"context"
	"net/http"
	"path"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

func (s *Service) CompleteUpload(ctx context.Context, userID, id string) (DTO, error) {
	record, err := s.Client.FileObject.Get(ctx, id)
	if err != nil || record.OwnerID != userID {
		return DTO{}, app.NotFound()
	}

	if record.Status == enum.FileReady {
		return toDTO(record), nil
	}

	if record.UploadExpiresAt == nil || !record.UploadExpiresAt.After(s.Now()) {
		return DTO{}, app.Conflict("上传已过期，请重新上传")
	}

	if s.Store == nil {
		return DTO{}, unavailable("TOS 文件存储尚未配置")
	}

	source := objectstore.Reference{
		Bucket: record.Bucket,
		Key:    record.UploadKey,
	}
	metadata, err := s.Store.Inspect(ctx, source)
	if err != nil {
		return DTO{}, unavailable("上传尚未完成或暂时无法核验，请重试")
	}

	if metadata.Size != record.Size || metadata.Size < 1 || metadata.Size > MaximumSize ||
		metadata.MediaType != string(record.MediaType) || metadata.ETag == "" {
		return DTO{}, app.Invalid("实际文件大小或类型与上传声明不一致")
	}

	source.VersionID = metadata.VersionID
	prefix, err := s.Store.ReadPrefix(ctx, source, metadata.ETag)
	if err != nil {
		return DTO{}, unavailable("文件内容核验失败，请重试")
	}

	if http.DetectContentType(prefix) != string(record.MediaType) {
		return DTO{}, app.Invalid("文件内容与声明类型不一致")
	}

	// Every completion attempt gets a separate private object. Concurrent or
	// replayed uploads can only modify the staging key, never an accepted file.
	destination := objectstore.Reference{
		Bucket: record.Bucket,
		Key:    path.Join(path.Dir(path.Dir(record.UploadKey)), "objects", app.NewID()+extension(record.MediaType)),
	}
	copied, err := s.Store.Copy(ctx, objectstore.CopyRequest{
		Source:      source,
		SourceETag:  metadata.ETag,
		Destination: destination,
		MediaType:   record.MediaType,
		FileName:    record.Name,
	})
	if err != nil || copied.ETag == "" {
		return DTO{}, unavailable("文件确认失败，请重试；不会接受尚未核验的文件")
	}

	destination.VersionID = copied.VersionID
	accepted, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (*ent.FileObject, error) {
		current, err := tx.FileObject.Query().
			Where(fileobject.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return nil, err
		}

		if current.Status == enum.FileReady {
			return current, nil
		}

		return tx.FileObject.UpdateOneID(id).
			SetStatus(enum.FileReady).
			SetObjectKey(destination.Key).
			SetVersionID(destination.VersionID).
			SetEtag(copied.ETag).
			SetVerifiedAt(s.Now()).
			Save(ctx)
	})
	if err != nil {
		// A failed commit may already have committed remotely. Keep the object;
		// deleting it here could remove a file already referenced by the database.
		return DTO{}, err
	}

	if accepted.ObjectKey != destination.Key {
		if err := s.Store.Delete(ctx, destination); err != nil {
			logging.From(ctx).Warn().Str("file_id", id).Msg("清理并发上传的未引用对象失败，需后续清理")
		}
	}

	return toDTO(accepted), nil
}
