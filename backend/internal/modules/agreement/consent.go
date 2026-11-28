package agreement

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/agreement"
	"baituber.cn/yanxuan/v2/ent/consent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) CurrentAgreement(ctx context.Context) (*ent.Agreement, error) {
	return Current(ctx, s.Client)
}

func (s *Service) Consent(ctx context.Context, userID string) error {
	current, err := s.CurrentAgreement(ctx)
	if err != nil {
		return app.WrapMissing(err)
	}

	return s.ConsentVersion(ctx, userID, current.ID)
}

func (s *Service) ConsentVersion(ctx context.Context, userID, agreementID string) error {
	if agreementID == "" {
		return app.Invalid("请确认正在阅读的协议版本")
	}

	_, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		currentUser, err := tx.User.Query().
			Where(user.IDEQ(userID)).
			ForUpdate().
			WithEnrollment(enrollmentstate.LoadCurrent).
			Only(ctx)
		if err != nil {
			return false, app.WrapMissing(err)
		}

		enrollment := enrollmentstate.Current(currentUser)
		if enrollment == nil || enrollment.Status != enum.EnrollmentApproved {
			return false, app.Forbidden("报名审核通过后方可签署")
		}

		return true, ConfirmCurrentVersion(ctx, tx, userID, agreementID)
	})
	return err
}

func ConfirmCurrentVersion(ctx context.Context, tx *ent.Tx, userID, agreementID string) error {
	if err := CheckCurrentVersion(ctx, tx.Agreement.Query().ForShare(), agreementID); err != nil {
		return err
	}

	exists, err := HasConsent(ctx, tx.Client(), userID, agreementID)
	if err != nil || exists {
		return err
	}

	err = tx.Consent.Create().
		SetUserID(userID).
		SetAgreementID(agreementID).
		Exec(ctx)
	if err != nil {
		return err
	}

	return app.Audit(ctx, tx, userID, enum.AuditAgreementConsent, agreementID, "", "协议编号："+agreementID)
}

func HasConsent(ctx context.Context, client *ent.Client, userID, agreementID string) (bool, error) {
	return client.Consent.Query().
		Where(
			consent.UserIDEQ(userID),
			consent.AgreementIDEQ(agreementID),
		).
		Exist(ctx)
}

func CheckCurrentVersion(ctx context.Context, query *ent.AgreementQuery, agreementID string) error {
	current, err := activeAgreement(query).First(ctx)
	if ent.IsNotFound(err) {
		return app.Conflict("当前营销分发任务协议尚未发布")
	}

	if err != nil {
		return err
	}

	if current.ID != agreementID {
		return app.Conflict("协议已更新，请重新阅读并确认")
	}

	return nil
}

func Current(ctx context.Context, client *ent.Client) (*ent.Agreement, error) {
	return activeAgreement(client.Agreement.Query()).First(ctx)
}

func activeAgreement(query *ent.AgreementQuery) *ent.AgreementQuery {
	return query.
		Where(agreement.ActiveEQ(true)).
		Order(ent.Desc(agreement.FieldCreatedAt))
}
