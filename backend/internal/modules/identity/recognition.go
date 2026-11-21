package identity

import (
	"context"
	"net/http"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type RecognizedIdentity struct {
	Name     string `json:"name"`
	Identity string `json:"identity"`
}

func (s *Service) RecognizeIdentity(ctx context.Context, userID, fileID string) (RecognizedIdentity, error) {
	photo, err := s.Client.FileObject.Query().
		Where(fileobject.IDEQ(fileID), ownedIdentityPhoto(userID)).
		Only(ctx)
	if err != nil {
		return RecognizedIdentity{}, app.WrapMissing(err)
	}

	if s.IdentityOCR == nil || s.Store == nil {
		return RecognizedIdentity{}, &app.Error{
			Status:  http.StatusServiceUnavailable,
			Code:    enum.ErrorUnavailable,
			Message: "身份证自动识别尚未配置",
		}
	}

	url, err := s.Store.SignDownload(ctx, objectstore.Reference{
		Bucket:    photo.Bucket,
		Key:       photo.ObjectKey,
		VersionID: photo.VersionID,
	}, 2*time.Minute)
	if err != nil {
		return RecognizedIdentity{}, &app.Error{
			Status:  http.StatusServiceUnavailable,
			Code:    enum.ErrorUnavailable,
			Message: "暂时无法读取身份证照片，请重试",
		}
	}

	name, number, err := s.IdentityOCR.Recognize(ctx, url)
	if err != nil {
		return RecognizedIdentity{}, err
	}

	return RecognizedIdentity{
		Name:     strings.TrimSpace(name),
		Identity: strings.ToUpper(strings.TrimSpace(number)),
	}, nil
}
