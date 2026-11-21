package identity

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) Distributors(ctx context.Context, userID string, revealPhone bool) ([]UserDTO, error) {
	query := s.Client.User.Query().
		Where(
			user.HasEnrollmentWith(
				userenrollment.IsCurrentEQ(true),
				userenrollment.StatusEQ(enum.EnrollmentApproved),
			),
		)
	if userID != "" {
		query.Where(user.IDEQ(userID))
	}

	members, err := query.
		Order(ent.Desc(user.FieldCreatedAt, user.FieldID)).
		WithVerification().
		WithPlatformProfile().
		WithEnrollment(enrollmentstate.LoadCurrent).
		All(ctx)
	if err != nil {
		return nil, err
	}

	if userID != "" && len(members) == 0 {
		return nil, app.NotFound()
	}

	userIDs := make([]string, 0, len(members))
	var blockedIDs []string
	for _, member := range members {
		userIDs = append(userIDs, member.ID)
		if member.Blocked {
			blockedIDs = append(blockedIDs, member.ID)
		}
	}

	blockEvents, err := s.latestBlockEvents(ctx, blockedIDs)
	if err != nil {
		return nil, err
	}

	consented, err := s.consentedUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	profiles := make([]UserDTO, 0, len(members))
	for _, member := range members {
		enrollment := enrollmentstate.Current(member)
		profile, err := s.userProfile(member, enrollment, consented[member.ID], true)
		if err != nil {
			return nil, err
		}

		profile.Nickname = member.Nickname
		if verification := member.Edges.Verification; verification != nil {
			profile.Wechat = s.Decrypt(verification.WechatCipher)
			if name := s.Decrypt(verification.NameCipher); name != "" {
				profile.Name = name
			}
		}

		event := blockEvents[member.ID]
		if event != nil {
			profile.BlockInfo = &DistributorBlockDTO{
				Reason: event.Reason,
				At:     event.CreatedAt,
			}
		}

		if userID != "" {
			profile.DistributorDetails = s.distributorDetails(member, enrollment, profile.Phone, revealPhone)
			if event != nil {
				operator, err := s.Client.Operator.Get(ctx, event.ActorID)
				if err != nil && !ent.IsNotFound(err) {
					return nil, err
				}

				if operator != nil {
					profile.BlockInfo.Operator = operator.Username
				}
			}
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func (s *Service) distributorDetails(member *ent.User, enrollment *ent.UserEnrollment, maskedPhone string, revealPhone bool) *DistributorDetailsDTO {
	details := &DistributorDetailsDTO{
		Phone:     maskedPhone,
		UpdatedAt: member.UpdatedAt,
	}
	verification := member.Edges.Verification
	if verification != nil {
		details.Identity = MaskIdentityNumber(s.Decrypt(verification.IdentityCipher))
	}

	if !revealPhone || maskedPhone == "" {
		return details
	}

	var currentContact, applicationContact, verifiedPhone string
	if verification != nil {
		currentContact = verification.ContactPhoneCipher
		verifiedPhone = verification.VerifiedPhoneCipher
	}

	if enrollment != nil {
		applicationContact = enrollment.ContactPhoneCipher
	}

	for _, cipher := range []string{currentContact, applicationContact, verifiedPhone} {
		phone := s.Decrypt(cipher)
		if MaskPhone(phone) == maskedPhone {
			details.Phone = phone
			break
		}
	}

	return details
}

func (s *Service) latestBlockEvents(ctx context.Context, userIDs []string) (map[string]*ent.AuditEvent, error) {
	result := make(map[string]*ent.AuditEvent, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	query := s.Client.AuditEvent.Query().
		Where(
			auditevent.ReferenceIDIn(userIDs...),
			auditevent.ActionEQ(enum.AuditUserBlock),
		).
		Order(ent.Desc(auditevent.FieldCreatedAt, auditevent.FieldID))
	if len(userIDs) == 1 {
		query.Limit(1)
	}

	events, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if result[event.ReferenceID] == nil {
			result[event.ReferenceID] = event
		}
	}

	return result, nil
}
