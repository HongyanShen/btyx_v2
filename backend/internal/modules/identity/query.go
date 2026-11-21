package identity

import (
	"context"
	"math"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/consent"
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	agreements "baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
)

const enrollmentPageSize = 30

func (s *Service) User(ctx context.Context, userID string) (UserDTO, error) {
	members, err := s.userProfiles(ctx, s.Client.User.Query().
		Where(user.IDEQ(userID)), false)
	if err != nil {
		return UserDTO{}, err
	}

	if len(members) == 0 {
		return UserDTO{}, app.NotFound()
	}

	profile := members[0]
	if profile.Blocked {
		events, err := s.latestBlockEvents(ctx, []string{userID})
		if err != nil {
			return UserDTO{}, err
		}

		if event := events[userID]; event != nil {
			profile.BlockReason = event.Reason
		}
	}

	if !profile.IdentityVerified {
		profile.IdentityResubmissionRequired, err = s.Client.AuditEvent.Query().
			Where(
				auditevent.ReferenceIDEQ(userID),
				auditevent.ActionIn(enum.AuditUserIdentityVerify, enum.AuditEnrollmentApproved),
			).
			Exist(ctx)
		if err != nil {
			return UserDTO{}, err
		}
	}

	return profile, nil
}

func (s *Service) Users(ctx context.Context) ([]UserDTO, error) {
	return s.userProfiles(ctx, s.Client.User.Query().
		Order(ent.Desc(user.FieldCreatedAt)).
		Limit(200), true)
}

func (s *Service) EnrollmentUsers(ctx context.Context, before string) ([]UserDTO, error) {
	query := s.enrollmentQuery()
	if before != "" {
		cursor, err := query.Clone().Where(userenrollment.IDEQ(before)).Only(ctx)
		if ent.IsNotFound(err) {
			cursor, err = query.Clone().Where(userenrollment.UserIDEQ(before), userenrollment.IsCurrentEQ(true)).Only(ctx)
		}

		if err != nil {
			return nil, app.WrapMissing(err)
		}

		query.Where(userenrollment.Or(
			userenrollment.CreatedAtLT(cursor.CreatedAt),
			userenrollment.And(
				userenrollment.CreatedAtEQ(cursor.CreatedAt),
				userenrollment.IDLT(cursor.ID),
			),
		))
	}

	return s.enrollmentProfiles(ctx, query.Limit(enrollmentPageSize+1))
}

func (s *Service) EnrollmentPage(ctx context.Context, page int, status enum.EnrollmentStatus, tag string) ([]UserDTO, int, error) {
	if page < 1 || page > math.MaxInt/enrollmentPageSize {
		return nil, 0, app.Invalid("分页参数无效")
	}

	query := s.enrollmentQuery()
	if status != "" {
		if !status.Valid() || status == enum.EnrollmentNone {
			return nil, 0, app.Invalid("报名状态无效")
		}

		query.Where(userenrollment.StatusEQ(status))
	}

	if tag != "" {
		if len(tag) > 512 {
			return nil, 0, app.Invalid("筛选标签无效")
		}

		query.Where(func(selector *sql.Selector) {
			selector.Where(sqljson.ValueContains(selector.C(userenrollment.FieldTags), tag))
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * enrollmentPageSize
	if offset >= total {
		return []UserDTO{}, total, nil
	}

	users, err := s.enrollmentProfiles(ctx, query.Offset(offset).Limit(enrollmentPageSize))
	return users, total, err
}

func (s *Service) enrollmentQuery() *ent.UserEnrollmentQuery {
	return s.Client.UserEnrollment.Query().
		Where(
			userenrollment.StatusNEQ(enum.EnrollmentNone),
			userenrollment.HasUserWith(mixin.NotDeleted),
		)
}

func (s *Service) enrollmentProfiles(ctx context.Context, query *ent.UserEnrollmentQuery) ([]UserDTO, error) {
	records, err := query.
		Order(ent.Desc(userenrollment.FieldCreatedAt, userenrollment.FieldID)).
		WithUser(func(query *ent.UserQuery) {
			query.WithVerification().WithPlatformProfile()
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, 0, len(records))
	var legacyUserIDs []string
	for _, record := range records {
		userIDs = append(userIDs, record.UserID)
		if record.IsCurrent && record.SubmittedAt == nil {
			legacyUserIDs = append(legacyUserIDs, record.UserID)
		}
	}

	submissionDates, err := s.legacyEnrollmentSubmissionDates(ctx, legacyUserIDs)
	if err != nil {
		return nil, err
	}

	consented, err := s.consentedUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	result := make([]UserDTO, 0, len(records))
	for _, record := range records {
		profile, err := s.userProfile(record.Edges.User, record, consented[record.UserID], true)
		if err != nil {
			return nil, err
		}

		profile.Name = MaskIdentityName(s.Decrypt(record.NameCipher))
		profile.Nickname = record.ApplicantNickname

		if submittedAt, exists := submissionDates[record.UserID]; exists && record.IsCurrent && record.SubmittedAt == nil {
			profile.EnrollmentSubmittedAt = &submittedAt
		}

		result = append(result, profile)
	}

	return result, nil
}

func (s *Service) legacyEnrollmentSubmissionDates(ctx context.Context, userIDs []string) (map[string]time.Time, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	var dates []struct {
		UserID      string    `json:"reference_id"`
		SubmittedAt time.Time `json:"max"`
	}
	err := s.Client.AuditEvent.Query().
		Where(
			auditevent.ReferenceIDIn(userIDs...),
			auditevent.ActionEQ(enum.AuditEnrollmentApply),
		).
		GroupBy(auditevent.FieldReferenceID).
		Aggregate(ent.Max(auditevent.FieldCreatedAt)).
		Scan(ctx, &dates)
	if err != nil {
		return nil, err
	}

	result := make(map[string]time.Time, len(dates))
	for _, date := range dates {
		result[date.UserID] = date.SubmittedAt
	}

	return result, nil
}

func (s *Service) userProfiles(ctx context.Context, query *ent.UserQuery, admin bool) ([]UserDTO, error) {
	members, err := query.
		WithVerification().
		WithPlatformProfile().
		WithEnrollment(enrollmentstate.LoadCurrent).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]UserDTO, 0, len(members))
	userIDs := make([]string, len(members))
	for index, member := range members {
		userIDs[index] = member.ID
	}

	consented, err := s.consentedUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for _, member := range members {
		profile, err := s.userProfile(member, enrollmentstate.Current(member), consented[member.ID], admin)
		if err != nil {
			return nil, err
		}

		result = append(result, profile)
	}

	return result, nil
}

func (s *Service) consentedUsers(ctx context.Context, userIDs []string) (map[string]bool, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	consented := make(map[string]bool)
	currentAgreement, err := agreements.Current(ctx, s.Client)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if currentAgreement != nil {
		records, err := s.Client.Consent.Query().
			Where(consent.AgreementIDEQ(currentAgreement.ID), consent.UserIDIn(userIDs...)).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			consented[record.UserID] = true
		}
	}

	return consented, nil
}

func (s *Service) userProfile(member *ent.User, enrollment *ent.UserEnrollment, consented, admin bool) (UserDTO, error) {
	nickname := member.Nickname
	if nickname == "" {
		var err error
		nickname, err = defaultNickname(member.ID)
		if err != nil {
			return UserDTO{}, err
		}
	}

	profile := UserDTO{
		ID:               member.ID,
		Nickname:         nickname,
		AvatarFileID:     member.AvatarFileID,
		Status:           enum.EnrollmentNone,
		Blocked:          member.Blocked,
		Consented:        consented,
		IdentityVerified: IsVerified(member),
	}

	var contactPhone string
	if verification := member.Edges.Verification; verification != nil {
		if !admin || profile.IdentityVerified {
			profile.Name = s.Decrypt(verification.NameCipher)
		}

		profile.VerifiedPhone = MaskPhone(s.Decrypt(verification.VerifiedPhoneCipher))
		contactPhone = MaskPhone(s.Decrypt(verification.ContactPhoneCipher))
		if !admin {
			profile.ContactPhone = contactPhone
			profile.Wechat = s.Decrypt(verification.WechatCipher)
			if profile.IdentityVerified {
				number := MaskIdentityNumber(s.Decrypt(verification.IdentityCipher))
				if number != "" || verification.FrontFileID != "" || verification.BackFileID != "" {
					profile.IdentityDocument = &VerifiedIdentityDTO{
						Number:      number,
						FrontFileID: verification.FrontFileID,
						BackFileID:  verification.BackFileID,
					}
				}
			}
		}
	}

	if platforms := member.Edges.PlatformProfile; platforms != nil && (enrollment == nil || enrollment.IsCurrent) {
		profile.Douyin = platforms.Douyin
		profile.XHS = platforms.Xhs
	}

	profile.Phone = profile.VerifiedPhone
	if enrollment != nil {
		if enrollment.Douyin != nil {
			profile.Douyin = *enrollment.Douyin
		}

		if enrollment.Xhs != nil {
			profile.XHS = *enrollment.Xhs
		}

		if enrollment.ContactPhoneCipher != "" {
			profile.Phone = MaskPhone(s.Decrypt(enrollment.ContactPhoneCipher))
		}

		profile.Status = enrollment.Status
		profile.Reason = enrollment.Reason
		profile.Tags = enrollment.Tags
		if enrollment.Status != enum.EnrollmentNone {
			profile.EnrollmentID = enrollment.ID
		}

		if !admin && !profile.IdentityVerified && enrollment.NameCipher != "" {
			profile.Name = s.Decrypt(enrollment.NameCipher)
		}

		if !admin && !profile.IdentityVerified && enrollment.Status == enum.EnrollmentRejected && enrollment.IdentityCheckedAt != nil {
			profile.EnrollmentDocument = &VerifiedIdentityDTO{
				Number:      MaskIdentityNumber(s.Decrypt(enrollment.IdentityCipher)),
				FrontFileID: enrollment.FrontFileID,
				BackFileID:  enrollment.BackFileID,
			}
		}

		if admin && enrollment.Status != enum.EnrollmentNone {
			if enrollment.IdentityCheckedAt != nil && enrollment.IsCurrent && profile.IdentityVerified {
				profile.Name = s.Decrypt(enrollment.NameCipher)
			}

			profile.EnrollmentUpdatedAt = &enrollment.UpdatedAt
			profile.EnrollmentSubmittedAt = enrollment.SubmittedAt
			if profile.EnrollmentSubmittedAt == nil {
				profile.EnrollmentSubmittedAt = &enrollment.CreatedAt
			}

			profile.EnrollmentHasPhotos = enrollment.FrontFileID != "" && enrollment.BackFileID != ""
			if verification := member.Edges.Verification; verification != nil && profile.IdentityVerified && enrollment.IsCurrent && enrollment.IdentityCheckedAt == nil {
				profile.EnrollmentHasPhotos = verification.FrontFileID != "" && verification.BackFileID != ""
			}

			profile.EnrollmentRecentPost = enrollment.RecentPost
			profile.EnrollmentNewAccount = enrollment.NewAccount
			profile.EnrollmentAgeGroup = enrollment.AgeGroup
		}

		if !enrollment.IsCurrent {
			profile.Phone = MaskPhone(s.Decrypt(enrollment.ContactPhoneCipher))
		}
	}

	if contactPhone != "" {
		profile.Phone = contactPhone
	}

	return profile, nil
}
