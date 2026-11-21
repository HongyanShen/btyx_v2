package agreement

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func CheckEligibility(ctx context.Context, tx *ent.Tx, userID string) error {
	_, err := EligibleEnrollment(ctx, tx, userID)
	return err
}

func EligibleEnrollment(ctx context.Context, tx *ent.Tx, userID string) (*ent.UserEnrollment, error) {
	currentUser, err := tx.User.Query().
		Where(user.IDEQ(userID)).
		WithEnrollment(enrollmentstate.LoadCurrent).
		Only(ctx)
	if err != nil {
		return nil, app.WrapMissing(err)
	}

	enrollment := enrollmentstate.Current(currentUser)
	if currentUser.Blocked || enrollment == nil || enrollment.Status != enum.EnrollmentApproved {
		return nil, app.Forbidden("需报名审核通过且账号正常")
	}

	currentAgreement, err := Current(ctx, tx.Client())
	if err != nil {
		return nil, app.Conflict("当前协议尚未配置")
	}

	ok, err := HasConsent(ctx, tx.Client(), userID, currentAgreement.ID)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, app.Forbidden("请先同意当前版本协议")
	}

	return enrollment, nil
}
