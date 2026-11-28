package main

import (
	"context"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

func browserIdentityFixtures(t *testing.T, ctx context.Context, services *business.Service) {
	t.Helper()

	member := browserIdentityMember(t, ctx, services, "新核验分发员")
	history := browserIdentityMember(t, ctx, services, "历史核验用户")
	services.AuditService.Store = browserIdentityStore{}
	front := browserIdentityPhoto(t, ctx, services, member, "identity-front")
	back := browserIdentityPhoto(t, ctx, services, member, "identity-back")
	wechat, err := services.Encrypt("example_wechat")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 11, 18, 10, 0, 0, 0, beijing)
	err = services.Client.AuditEvent.Create().
		SetActorID(member).
		SetReferenceID(member).
		SetCreatedAt(base).
		SetAction(enum.AuditUserIdentityVerify).
		SetSubmittedMaskedName("张**").
		SetSubmittedMaskedIdentity("5101**********1234").
		SetSubmittedPhone("138****8888").
		SetSubmittedWechatCipher(wechat).
		SetSubmittedFrontFileID(front).
		SetSubmittedBackFileID(back).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = services.Client.AuditEvent.Create().
		SetActorID(history).
		SetReferenceID(history).
		SetCreatedAt(base.Add(-time.Minute)).
		SetAction(enum.AuditEnrollmentIdentityFailed).
		SetReason("姓名与身份证号不匹配").
		SetSubmittedPhone("139****1111").
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for index := range 49 {
		reason := "核验请求失败"
		if index == 48 {
			reason = "较早核验记录"
		}

		err := services.Client.AuditEvent.Create().
			SetActorID(history).
			SetReferenceID(history).
			SetCreatedAt(base.Add(-time.Duration(index+2) * time.Minute)).
			SetAction(enum.AuditUserIdentityFailed).
			SetReason(reason).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func browserIdentityMember(t *testing.T, ctx context.Context, services *business.Service, nickname string) string {
	t.Helper()

	member, err := services.DemoLogin(ctx, business.NewID())
	if err != nil {
		t.Fatal(err)
	}

	if err := services.UpdateProfile(ctx, member.UserID, identity.UpdateProfileInput{Nickname: &nickname}); err != nil {
		t.Fatal(err)
	}

	return member.UserID
}

func browserIdentityPhoto(t *testing.T, ctx context.Context, services *business.Service, member, key string) string {
	t.Helper()

	file, err := services.Client.FileObject.Create().
		SetOwnerID(member).
		SetName("identity-fixture.png").
		SetBucket("browser-fixture").
		SetObjectKey(key).
		SetVersionID("frozen").
		SetStorage(enum.FileStorageTOS).
		SetStatus(enum.FileReady).
		SetMediaType(enum.FilePNG).
		SetSize(100).
		SetVerifiedAt(services.Now()).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return file.ID
}

type browserIdentityStore struct {
	objectstore.Gateway
}

func (browserIdentityStore) SignDownload(_ context.Context, reference objectstore.Reference, _ time.Duration) (string, error) {
	return "https://images.example.test/" + reference.Key, nil
}
