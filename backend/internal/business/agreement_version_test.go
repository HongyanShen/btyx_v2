package business

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestConsentRejectsAgreementChangedAfterReading(t *testing.T) {
	services := testService(t)
	ctx := context.Background()
	login, err := services.DemoLogin(ctx, "agreement-version")
	if err != nil {
		t.Fatal(err)
	}

	err = services.Client.UserEnrollment.Create().
		SetUserID(login.UserID).
		SetStatus(enum.EnrollmentApproved).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	old, err := services.CurrentAgreement(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := services.PublishAgreement(ctx, "admin", "新版协议", "更新后的合作条件"); err != nil {
		t.Fatal(err)
	}

	if err := services.ConsentVersion(ctx, login.UserID, old.ID); err == nil {
		t.Fatal("accepted agreement that the user has not read")
	}

	count, err := services.Client.Consent.Query().Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("stale consent wrote records: count=%d, err=%v", count, err)
	}

	current, err := services.CurrentAgreement(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := services.ConsentVersion(ctx, login.UserID, current.ID); err != nil {
		t.Fatal(err)
	}
}
