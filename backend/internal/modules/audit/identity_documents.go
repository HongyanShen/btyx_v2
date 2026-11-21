package audit

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/predicate"
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type IdentityRecordDetailsDTO struct {
	Name     string `json:"name"`
	Identity string `json:"identity"`
	Phone    string `json:"phone"`
	Wechat   string `json:"wechat"`
	FrontURL string `json:"front_url"`
	BackURL  string `json:"back_url"`
}

type EnrollmentDocumentsDTO struct {
	Details           *IdentityRecordDetailsDTO `json:"details"`
	PhotosUnavailable bool                      `json:"photos_unavailable"`
}

func (s *Service) EnrollmentDocuments(ctx context.Context, userID, enrollmentID string) (EnrollmentDocumentsDTO, error) {
	query := s.enrollmentDocumentsQuery().Where(userenrollment.UserIDEQ(userID))
	if enrollmentID == "" {
		query.Where(userenrollment.IsCurrentEQ(true))
	} else {
		query.Where(userenrollment.IDEQ(enrollmentID))
	}

	application, err := query.Only(ctx)
	if err != nil {
		return EnrollmentDocumentsDTO{}, app.WrapMissing(err)
	}

	document := identityDocumentForEnrollment(application)
	photos, err := s.loadIdentityPhotos(
		ctx,
		[]string{document.FrontFileID, document.BackFileID},
		fileobject.IdentityPrivateEQ(false),
	)
	if err != nil {
		return EnrollmentDocumentsDTO{}, err
	}

	front, frontFailed := s.signIdentityPhoto(ctx, photos[document.FrontFileID], userID)
	back, backFailed := s.signIdentityPhoto(ctx, photos[document.BackFileID], userID)
	profile := &IdentityRecordDetailsDTO{
		Name:     identity.MaskIdentityName(s.Decrypt(document.NameCipher)),
		Identity: identity.MaskIdentityNumber(s.Decrypt(document.IdentityCipher)),
		FrontURL: front,
		BackURL:  back,
	}
	var verifiedPhone string
	if verification := application.Edges.User.Edges.Verification; verification != nil && application.IsCurrent {
		profile.Wechat = s.Decrypt(verification.WechatCipher)
		profile.Phone = identity.MaskPhone(s.Decrypt(verification.ContactPhoneCipher))
		verifiedPhone = identity.MaskPhone(s.Decrypt(verification.VerifiedPhoneCipher))
	}

	if profile.Phone == "" {
		profile.Phone = identity.MaskPhone(s.Decrypt(application.ContactPhoneCipher))
	}

	if profile.Phone == "" {
		profile.Phone = verifiedPhone
	}

	return EnrollmentDocumentsDTO{
		Details:           profile,
		PhotosUnavailable: frontFailed || backFailed,
	}, nil
}

func (s *Service) EnrollmentPhotoPreviews(ctx context.Context, enrollmentIDs []string) (map[string]*identity.EnrollmentPhotoPreviewDTO, error) {
	if len(enrollmentIDs) == 0 {
		return nil, nil
	}

	applications, err := s.enrollmentDocumentsQuery().
		Where(userenrollment.IDIn(enrollmentIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	documents := make(map[string]identity.Document, len(applications))
	var photoIDs []string
	for _, application := range applications {
		document := identityDocumentForEnrollment(application)
		if document.FrontFileID == "" && document.BackFileID == "" {
			continue
		}

		documents[application.ID] = document
		if document.FrontFileID != "" {
			photoIDs = append(photoIDs, document.FrontFileID)
		}

		if document.BackFileID != "" {
			photoIDs = append(photoIDs, document.BackFileID)
		}
	}

	photos, err := s.loadIdentityPhotos(ctx, photoIDs, fileobject.IdentityPrivateEQ(false))
	if err != nil {
		return nil, err
	}

	previews := make(map[string]*identity.EnrollmentPhotoPreviewDTO, len(documents))
	for _, application := range applications {
		document, exists := documents[application.ID]
		if !exists {
			continue
		}

		userID := application.UserID
		front, _ := s.signIdentityPhoto(ctx, photos[document.FrontFileID], userID)
		back, _ := s.signIdentityPhoto(ctx, photos[document.BackFileID], userID)
		previews[application.ID] = &identity.EnrollmentPhotoPreviewDTO{
			FrontURL: front,
			BackURL:  back,
			Unavailable: (document.FrontFileID != "" && front == "") ||
				(document.BackFileID != "" && back == ""),
		}
	}

	return previews, nil
}

func (s *Service) enrollmentDocumentsQuery() *ent.UserEnrollmentQuery {
	return s.Client.UserEnrollment.Query().
		Where(
			userenrollment.StatusNEQ(enum.EnrollmentNone),
			userenrollment.HasUserWith(mixin.NotDeleted),
		).
		WithUser(func(query *ent.UserQuery) {
			query.WithVerification()
		})
}

func (s *Service) loadIdentityPhotos(ctx context.Context, ids []string, filters ...predicate.FileObject) (map[string]*ent.FileObject, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	photos, err := s.Client.FileObject.Query().
		Where(
			fileobject.IDIn(ids...),
			identity.ReadyIdentityPhoto(),
		).
		Where(filters...).
		All(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*ent.FileObject, len(photos))
	for _, photo := range photos {
		byID[photo.ID] = photo
	}

	return byID, nil
}

func identityDocumentForEnrollment(application *ent.UserEnrollment) identity.Document {
	if application.IdentityCheckedAt != nil {
		return identity.Document{
			NameCipher:     application.NameCipher,
			IdentityCipher: application.IdentityCipher,
			FrontFileID:    application.FrontFileID,
			BackFileID:     application.BackFileID,
		}
	}

	if verification := application.Edges.User.Edges.Verification; application.IsCurrent && verification != nil && verification.VerifiedAt != nil {
		return identity.Document{
			NameCipher:     verification.NameCipher,
			IdentityCipher: verification.IdentityCipher,
			FrontFileID:    verification.FrontFileID,
			BackFileID:     verification.BackFileID,
		}
	}

	return identity.Document{}
}

func (s *Service) signIdentityPhoto(ctx context.Context, photo *ent.FileObject, userID string) (string, bool) {
	if photo == nil || photo.OwnerID != userID {
		return "", false
	}

	if s.Store == nil {
		return "", true
	}

	url, err := s.Store.SignDownload(ctx, objectstore.Reference{
		Bucket:    photo.Bucket,
		Key:       photo.ObjectKey,
		VersionID: photo.VersionID,
	}, 2*time.Minute)
	if err != nil || url == "" {
		return "", true
	}

	return url, false
}
