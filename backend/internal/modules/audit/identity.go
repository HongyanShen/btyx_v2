package audit

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type IdentityRecordDTO struct {
	ID        string                    `json:"id"`
	UserID    string                    `json:"user_id"`
	Nickname  string                    `json:"nickname"`
	Action    enum.AuditAction          `json:"action"`
	Reason    string                    `json:"reason"`
	CreatedAt time.Time                 `json:"created_at"`
	Details   *IdentityRecordDetailsDTO `json:"details"`
}

type IdentityRecordsDTO struct {
	Records           []IdentityRecordDTO `json:"records"`
	NextBefore        string              `json:"next_before"`
	PhotosUnavailable bool                `json:"photos_unavailable"`
}

func (s *Service) IdentityRecords(ctx context.Context, before string) (IdentityRecordsDTO, error) {
	return s.identityRecords(ctx, before, false)
}

func (s *Service) IdentityRecordsWithDocuments(ctx context.Context, before string) (IdentityRecordsDTO, error) {
	return s.identityRecords(ctx, before, true)
}

func (s *Service) identityRecords(ctx context.Context, before string, includeDetails bool) (IdentityRecordsDTO, error) {
	const pageSize = 50
	query := s.Client.AuditEvent.Query().
		Where(auditevent.ActionIn(
			enum.AuditUserIdentityVerify,
			enum.AuditUserIdentityFailed,
			enum.AuditEnrollmentIdentityVerify,
			enum.AuditEnrollmentIdentityFailed,
		))
	if before != "" {
		cursor, err := query.Clone().Where(auditevent.IDEQ(before)).Only(ctx)
		if ent.IsNotFound(err) {
			return IdentityRecordsDTO{}, app.Invalid("认证记录分页位置无效")
		}

		if err != nil {
			return IdentityRecordsDTO{}, err
		}

		query.Where(auditevent.Or(
			auditevent.CreatedAtLT(cursor.CreatedAt),
			auditevent.And(
				auditevent.CreatedAtEQ(cursor.CreatedAt),
				auditevent.IDLT(cursor.ID),
			),
		))
	}

	events, err := query.
		Order(ent.Desc(auditevent.FieldCreatedAt, auditevent.FieldID)).
		Limit(pageSize + 1).
		All(ctx)
	if err != nil {
		return IdentityRecordsDTO{}, err
	}

	result := IdentityRecordsDTO{Records: make([]IdentityRecordDTO, 0, len(events))}
	if len(events) == 0 {
		return result, nil
	}

	if len(events) > pageSize {
		events = events[:pageSize]
		result.NextBefore = events[pageSize-1].ID
	}

	userIDs := make([]string, 0, len(events))
	for _, event := range events {
		userIDs = append(userIDs, event.ReferenceID)
	}

	members, err := s.Client.User.Query().
		Where(user.IDIn(userIDs...)).
		Select(user.FieldID, user.FieldNickname).
		All(ctx)
	if err != nil {
		return IdentityRecordsDTO{}, err
	}

	nicknames := make(map[string]string, len(members))
	for _, member := range members {
		nicknames[member.ID] = member.Nickname
	}

	var photos map[string]*ent.FileObject
	if includeDetails {
		var photoIDs []string
		for _, event := range events {
			if event.SubmittedFrontFileID != "" {
				photoIDs = append(photoIDs, event.SubmittedFrontFileID)
			}

			if event.SubmittedBackFileID != "" {
				photoIDs = append(photoIDs, event.SubmittedBackFileID)
			}
		}

		// A historical snapshot may still reference a photo retired from the current profile.
		photos, err = s.loadIdentityPhotos(ctx, photoIDs)
		if err != nil {
			return IdentityRecordsDTO{}, err
		}
	}

	for _, event := range events {
		var recordDetails *IdentityRecordDetailsDTO
		nickname, memberExists := nicknames[event.ReferenceID]
		if includeDetails && memberExists {
			front, frontFailed := s.signIdentityPhoto(ctx, photos[event.SubmittedFrontFileID], event.ReferenceID)
			back, backFailed := s.signIdentityPhoto(ctx, photos[event.SubmittedBackFileID], event.ReferenceID)
			recordDetails = &IdentityRecordDetailsDTO{
				Name:     event.SubmittedMaskedName,
				Identity: event.SubmittedMaskedIdentity,
				Phone:    event.SubmittedPhone,
				Wechat:   s.Decrypt(event.SubmittedWechatCipher),
				FrontURL: front,
				BackURL:  back,
			}
			result.PhotosUnavailable = result.PhotosUnavailable || frontFailed || backFailed
		}

		result.Records = append(result.Records, IdentityRecordDTO{
			ID:        event.ID,
			UserID:    event.ReferenceID,
			Nickname:  nickname,
			Action:    event.Action,
			Reason:    event.Reason,
			CreatedAt: event.CreatedAt,
			Details:   recordDetails,
		})
	}

	return result, nil
}
