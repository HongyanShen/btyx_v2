package audit

import (
	"context"
	"slices"
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

type IdentityRecordPage struct {
	IdentityRecordsDTO
	PreviousAfter string
	Number        int
}

func (s *Service) IdentityRecords(ctx context.Context, before string) (IdentityRecordsDTO, error) {
	page, err := s.identityRecords(ctx, before, "", 0, false)
	return page.IdentityRecordsDTO, err
}

func (s *Service) IdentityRecordsWithDocuments(ctx context.Context, before string) (IdentityRecordsDTO, error) {
	page, err := s.identityRecords(ctx, before, "", 0, true)
	return page.IdentityRecordsDTO, err
}

func (s *Service) IdentityRecordsPage(ctx context.Context, before, after string, page int, includeDetails bool) (IdentityRecordPage, error) {
	return s.identityRecords(ctx, before, after, max(page, 1), includeDetails)
}

func (s *Service) identityRecords(ctx context.Context, before, after string, page int, includeDetails bool) (IdentityRecordPage, error) {
	if before != "" && after != "" {
		return IdentityRecordPage{}, app.Invalid("认证记录分页位置无效")
	}

	const pageSize = 50
	query := s.Client.AuditEvent.Query().
		Where(auditevent.ActionIn(
			enum.AuditUserIdentityVerify,
			enum.AuditUserIdentityFailed,
			enum.AuditEnrollmentIdentityVerify,
			enum.AuditEnrollmentIdentityFailed,
		))
	allRecords := query.Clone()
	cursorID := before
	if after != "" {
		cursorID = after
	}

	if cursorID == "" && page > 1 {
		count, err := allRecords.Clone().Count(ctx)
		if err != nil {
			return IdentityRecordPage{}, err
		}

		page = min(page, max(1, (count+pageSize-1)/pageSize))
		query.Offset((page - 1) * pageSize)
	}

	if cursorID != "" {
		cursor, err := query.Clone().Where(auditevent.IDEQ(cursorID)).Only(ctx)
		if ent.IsNotFound(err) {
			return IdentityRecordPage{}, app.Invalid("认证记录分页位置无效")
		}

		if err != nil {
			return IdentityRecordPage{}, err
		}

		if after != "" {
			query.Where(auditevent.Or(
				auditevent.CreatedAtGT(cursor.CreatedAt),
				auditevent.And(auditevent.CreatedAtEQ(cursor.CreatedAt), auditevent.IDGT(cursor.ID)),
			))
		} else {
			query.Where(auditevent.Or(
				auditevent.CreatedAtLT(cursor.CreatedAt),
				auditevent.And(auditevent.CreatedAtEQ(cursor.CreatedAt), auditevent.IDLT(cursor.ID)),
			))
		}
	}

	if after != "" {
		query.Order(ent.Asc(auditevent.FieldCreatedAt, auditevent.FieldID))
	} else {
		query.Order(ent.Desc(auditevent.FieldCreatedAt, auditevent.FieldID))
	}

	events, err := query.Limit(pageSize + 1).All(ctx)
	if err != nil {
		return IdentityRecordPage{}, err
	}

	result := IdentityRecordPage{Number: page}
	result.Records = make([]IdentityRecordDTO, 0, len(events))
	if len(events) == 0 {
		return result, nil
	}

	hasMore := len(events) > pageSize
	if hasMore {
		events = events[:pageSize]
	}

	if after != "" {
		slices.Reverse(events)
	}

	if before != "" || (after != "" && hasMore) {
		result.PreviousAfter = events[0].ID
	}

	if after != "" || hasMore {
		result.NextBefore = events[len(events)-1].ID
	}

	if page > 0 && cursorID != "" {
		first := events[0]
		newer, err := allRecords.Where(auditevent.Or(
			auditevent.CreatedAtGT(first.CreatedAt),
			auditevent.And(auditevent.CreatedAtEQ(first.CreatedAt), auditevent.IDGT(first.ID)),
		)).Count(ctx)
		if err != nil {
			return IdentityRecordPage{}, err
		}

		result.Number = newer/pageSize + 1
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
		return IdentityRecordPage{}, err
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
			return IdentityRecordPage{}, err
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
