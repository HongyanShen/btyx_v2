package files

import (
	"context"
	"path"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

const uploadLifetime = 10 * time.Minute

func (s *Service) InitiateUpload(ctx context.Context, userID string, role enum.Role, input InitiateInput, key string) (UploadTicket, error) {
	if err := app.ValidateKey(key); err != nil {
		return UploadTicket{}, err
	}

	input.Name = path.Base(strings.ReplaceAll(strings.TrimSpace(input.Name), "\\", "/"))
	if input.Name == "" || input.Name == "." || input.Name == "/" || len(input.Name) > 255 || strings.ContainsAny(input.Name, "\x00\r\n") {
		return UploadTicket{}, app.Invalid("文件名无效或过长")
	}

	if !input.MediaType.Valid() || input.Size < 1 || input.Size > MaximumSize {
		return UploadTicket{}, app.Invalid("仅支持25MB以内的 JPG、PNG、GIF、WEBP 图片及 MP4 视频")
	}

	if s.Store == nil {
		return UploadTicket{}, unavailable("TOS 文件存储尚未配置")
	}

	record, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (*ent.FileObject, error) {
		if role == enum.RoleAdmin {
			if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "file-upload:"+userID); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.User.Query().
				Where(user.IDEQ(userID)).
				ForUpdate().
				Only(ctx); err != nil {
				return nil, app.WrapMissing(err)
			}
		}

		requestKey := userID + ":" + key
		old, err := tx.FileObject.Query().Where(fileobject.RequestKeyEQ(requestKey)).Only(ctx)
		if err == nil {
			if old.Name != input.Name || old.Size != input.Size || old.MediaType != input.MediaType {
				return nil, app.Conflict("幂等键与原文件不一致")
			}

			return old, nil
		}

		if !ent.IsNotFound(err) {
			return nil, err
		}

		pending, err := tx.FileObject.Query().Where(
			fileobject.OwnerIDEQ(userID),
			fileobject.StatusEQ(enum.FilePending),
			fileobject.UploadExpiresAtGT(s.Now()),
		).Count(ctx)
		if err != nil {
			return nil, err
		}

		if pending >= 20 {
			return nil, app.Conflict("待完成的上传较多，请稍后再试")
		}

		id := app.NewID()
		return tx.FileObject.Create().
			SetID(id).
			SetOwnerID(userID).
			SetName(input.Name).
			SetMediaType(input.MediaType).
			SetSize(input.Size).
			SetStorage(enum.FileStorageTOS).
			SetStatus(enum.FilePending).
			SetBucket(s.Config.Storage.Bucket).
			SetUploadKey(s.Config.Storage.Prefix + "uploads/" + id + extension(input.MediaType)).
			SetRequestKey(requestKey).
			SetUploadExpiresAt(s.Now().Add(uploadLifetime)).
			Save(ctx)
	})
	if err != nil {
		return UploadTicket{}, err
	}

	if record.Status == enum.FileReady {
		return UploadTicket{
			ID:        record.ID,
			Completed: true,
		}, nil
	}

	if record.UploadExpiresAt == nil {
		return UploadTicket{}, app.Conflict("上传凭证已过期，请重新选择文件上传")
	}

	expires := record.UploadExpiresAt.Sub(s.Now())
	if expires < time.Second {
		return UploadTicket{}, app.Conflict("上传凭证已过期，请重新选择文件上传")
	}

	form, err := s.Store.SignUpload(ctx, objectstore.UploadRequest{
		Object: objectstore.Reference{
			Bucket: record.Bucket,
			Key:    record.UploadKey,
		},
		Size:      record.Size,
		MediaType: record.MediaType,
		Expires:   expires,
	})
	if err != nil {
		return UploadTicket{}, unavailable("暂时无法获取上传凭证，请重试")
	}

	return UploadTicket{
		ID:        record.ID,
		URL:       form.URL,
		Fields:    form.Fields,
		ExpiresAt: record.UploadExpiresAt,
	}, nil
}

func extension(mediaType enum.FileMediaType) string {
	switch mediaType {
	case enum.FileJPEG:
		return ".jpg"
	case enum.FilePNG:
		return ".png"
	case enum.FileGIF:
		return ".gif"
	case enum.FileWEBP:
		return ".webp"
	case enum.FileMP4:
		return ".mp4"
	default:
		return ""
	}
}
