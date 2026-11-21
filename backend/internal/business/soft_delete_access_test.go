package business

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func TestArchivedIdentityPhotosRemainExcludedFromGenericFileAccess(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	member, err := service.DemoLogin(ctx, "archive-identity-photo-owner")
	if err != nil {
		t.Fatal(err)
	}

	createPhoto := func(name string) string {
		t.Helper()

		photo, err := service.Client.FileObject.Create().
			SetOwnerID(member.UserID).
			SetName(name).
			SetMediaType(enum.FileJPEG).
			SetSize(100).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return photo.ID
	}

	verificationPhoto := createPhoto("verification.jpg")
	err = service.Client.UserVerification.Create().
		SetUserID(member.UserID).
		SetFrontFileID(verificationPhoto).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	enrollmentPhoto := createPhoto("enrollment.jpg")
	err = service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentRejected).
		SetBackFileID(enrollmentPhoto).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Client.User.DeleteOneID(member.UserID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	for _, photoID := range []string{verificationPhoto, enrollmentPhoto} {
		_, err := service.FileForRead(ctx, "admin", enum.RoleAdmin, photoID)
		var appError *app.Error
		if !errors.As(err, &appError) || appError.Status != http.StatusNotFound {
			t.Errorf("generic endpoint can access a photo referenced by archived account data: %v", err)
		}
	}
}
