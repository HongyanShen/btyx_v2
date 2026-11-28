package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/user"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatmini"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

type verifyPhoneFunc func(context.Context, string) (string, error)

func (verify verifyPhoneFunc) Number(ctx context.Context, code string) (string, error) {
	return verify(ctx, code)
}

func TestEntUpgradePreservesExistingMember(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	legacy, err := services.Client.User.Create().Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.DB.ExecContext(ctx, `
		ALTER TABLE users
			DROP COLUMN nickname,
			DROP COLUMN avatar_file_id,
			ADD COLUMN openid text NOT NULL UNIQUE DEFAULT 'legacy-account',
			ADD COLUMN name_cipher text NOT NULL DEFAULT '',
			ADD COLUMN identity_cipher text NOT NULL DEFAULT '',
			ADD COLUMN phone_cipher text NOT NULL DEFAULT '',
			ADD COLUMN douyin text NOT NULL DEFAULT '',
			ADD COLUMN xhs text NOT NULL DEFAULT '',
			ADD COLUMN enrollment_status text NOT NULL DEFAULT 'none',
			ADD COLUMN reason text NOT NULL DEFAULT '',
			ADD COLUMN tags jsonb;
		ALTER TABLE audit_events
			DROP CONSTRAINT audit_action_enum_v7,
			ADD CONSTRAINT audit_action_enum CHECK (action IN ('enrollment.apply', 'operator.create'));
	`); err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(ctx, services.Config); err != nil {
		t.Fatal(err)
	}

	profile, err := services.User(ctx, legacy.ID)
	if err != nil || !strings.HasPrefix(profile.Nickname, "微信用户") {
		t.Fatalf("existing member lacks an initial nickname: err=%v", err)
	}

	nickname := "老用户的新昵称"
	if err := services.UpdateProfile(ctx, legacy.ID, identity.UpdateProfileInput{Nickname: &nickname}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentFirstLoginSharesAccountAndWallet(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	const requests = 8
	type loginAttempt struct {
		userID string
		err    error
	}

	start := make(chan struct{})
	attempts := make(chan loginAttempt, requests)
	for range requests {
		go func() {
			<-start
			login, err := services.DemoLogin(ctx, "concurrent-signup")
			attempts <- loginAttempt{
				userID: login.UserID,
				err:    err,
			}
		}()
	}

	close(start)

	var userID string
	for range requests {
		result := <-attempts
		if result.err != nil {
			t.Error(result.err)

			continue
		}

		if userID == "" {
			userID = result.userID
		}

		if result.userID != userID {
			t.Error("concurrent logins registered multiple accounts")
		}
	}

	var users, identities, wallets int
	err := services.DB.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM users),
		       (SELECT count(*) FROM wechat_identities),
		       (SELECT count(*) FROM wallets)
	`).Scan(&users, &identities, &wallets)
	if err != nil || users != 1 || identities != 1 || wallets != 1 {
		t.Fatalf("registration left duplicate or orphaned records: users=%d identities=%d wallets=%d err=%v",
			users, identities, wallets, err)
	}
}

func TestReapplicationPreservesVerifiedPhone(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	terms, err := services.Client.Agreement.Create().
		SetTitle("营销分发任务协议").
		SetBody("报名须确认本协议").
		SetCreatorID("admin").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	services.EnrollmentService.Verifier = verifyDocumentFunc(func(context.Context, string, string) error {
		return nil
	})
	login, err := services.DemoLogin(ctx, "reapplication")
	if err != nil {
		t.Fatal(err)
	}

	services.WechatPhone = verifyPhoneFunc(func(context.Context, string) (string, error) {
		return "13900000000", nil
	})
	if _, err := services.VerifyPhone(ctx, login.UserID, "test-code", false); err != nil {
		t.Fatal(err)
	}

	input := enrollment.ApplyInput{
		DocumentInput: identity.DocumentInput{
			Name:        "测试报名",
			Identity:    adultIdentity,
			FrontFileID: addIdentityPhoto(t, services, login.UserID),
			BackFileID:  addIdentityPhoto(t, services, login.UserID),
		},
		Phone:       "13800000001",
		Douyin:      " old-dy ",
		XHS:         "old-xhs",
		AgreementID: terms.ID,
	}
	if err := services.Apply(ctx, login.UserID, input); err != nil {
		t.Fatal(err)
	}

	if err := services.ReviewEnrollment(ctx, "staff", login.UserID, false, "补充资料", nil, nil); err != nil {
		t.Fatal(err)
	}

	rejected, err := services.User(ctx, login.UserID)
	if err != nil {
		t.Fatal(err)
	}

	input.PreviousEnrollmentID = rejected.EnrollmentID
	input.Phone = "13800000002"
	input.Douyin = " new-dy "
	input.XHS = "new-xhs"
	if err := services.Apply(ctx, login.UserID, input); err != nil {
		t.Fatal(err)
	}

	profile, err := services.User(ctx, login.UserID)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Name != input.Name || profile.Phone != "138****0002" || profile.VerifiedPhone != "139****0000" ||
		profile.Douyin != "new-dy" || profile.XHS != input.XHS || profile.Status != enum.EnrollmentPending || profile.Reason != "" {
		t.Fatal("reapplication did not update the intended domains")
	}
}

func TestMemberProfileAndLogout(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	router := minihandler.NewRouter(services)
	member, err := services.DemoLogin(ctx, "profile-member")
	if err != nil {
		t.Fatal(err)
	}

	request := func(method, path, token, body string, want int) map[string]string {
		t.Helper()

		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: got HTTP %d, want %d: %s", method, path, response.Code, want, response.Body.String())
		}

		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}

		data := make(map[string]string)
		for key, value := range envelope.Data {
			var text string
			if json.Unmarshal(value, &text) == nil {
				data[key] = text
			}
		}

		return data
	}

	profile := request(http.MethodGet, "/me", member.Token, "", http.StatusOK)
	if !strings.HasPrefix(profile["nickname"], "微信用户") || len(strings.TrimPrefix(profile["nickname"], "微信用户")) < 4 {
		t.Fatalf("missing default Sqids nickname: %q", profile["nickname"])
	}

	other, err := services.DemoLogin(ctx, "profile-other")
	if err != nil {
		t.Fatal(err)
	}

	avatar, err := services.Client.FileObject.Create().
		SetOwnerID(member.UserID).
		SetName("avatar.png").
		SetMediaType(enum.FilePNG).
		SetSize(48).
		SetStorage(enum.FileStorageTOS).
		SetStatus(enum.FileReady).
		SetBucket("test").
		SetObjectKey("objects/avatar.png").
		SetVerifiedAt(services.Now()).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	nickname := strings.Repeat("兔", 31) + "🐰"
	body := `{"nickname":"` + nickname + `","avatar_file_id":"` + avatar.ID + `"}`
	request(http.MethodPut, "/me", other.Token, body, http.StatusBadRequest)
	request(http.MethodPut, "/me", member.Token, body, http.StatusOK)
	request(http.MethodPut, "/me", member.Token, body, http.StatusOK)

	count, err := services.Client.AuditEvent.Query().
		Where(auditevent.ActorIDEQ(member.UserID)).
		Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("identical profile update should audit once: count=%d, err=%v", count, err)
	}

	request(http.MethodPut, "/me", member.Token, `{"nickname":"`+nickname+"兔"+`"}`, http.StatusBadRequest)
	rejected, err := services.Client.AuditEvent.Query().
		Where(auditevent.ActorIDEQ(member.UserID), auditevent.ActionEQ(enum.AuditRequestFailed)).
		Only(ctx)
	if err != nil || rejected.Endpoint != "PUT /api/v1/me" {
		t.Fatalf("invalid profile update should have a separate HTTP failure audit: %v", err)
	}

	services.WechatPhone = wechatmini.NewPhoneClient("", "")
	request(http.MethodPost, "/me/phone", member.Token, `{"code":"test-code"}`, http.StatusServiceUnavailable)

	phoneCalls := 0
	services.WechatPhone = verifyPhoneFunc(func(_ context.Context, code string) (string, error) {
		phoneCalls++
		if code != "test-code" {
			t.Fatal("unexpected WeChat phone code")
		}

		return "13800000000", nil
	})
	request(http.MethodPost, "/me/phone", member.Token, `{"phone":"13800000000"}`, http.StatusBadRequest)
	verified := request(http.MethodPost, "/me/phone", member.Token, `{"code":"test-code"}`, http.StatusOK)
	if verified["verified_phone"] != "138****0000" || phoneCalls != 1 {
		t.Fatal("expected successful code exchange and masked phone")
	}

	record, err := services.Client.User.Query().
		Where(user.IDEQ(member.UserID)).
		WithVerification().
		WithEnrollment().
		Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	verification := record.Edges.Verification
	if verification == nil || verification.VerifiedPhoneCipher == "13800000000" ||
		services.Decrypt(verification.VerifiedPhoneCipher) != "13800000000" || len(record.Edges.Enrollment) != 0 {
		t.Fatal("verified phone was not stored separately as ciphertext")
	}

	returning, err := services.DemoLogin(ctx, "profile-member")
	if err != nil {
		t.Fatal(err)
	}

	request(http.MethodPost, "/auth/logout", member.Token, "", http.StatusOK)
	request(http.MethodGet, "/me", member.Token, "", http.StatusUnauthorized)
	request(http.MethodGet, "/me", other.Token, "", http.StatusOK)

	saved := request(http.MethodGet, "/me", returning.Token, "", http.StatusOK)
	if returning.UserID != member.UserID || saved["nickname"] != nickname ||
		saved["avatar_file_id"] != avatar.ID || saved["verified_phone"] != "138****0000" || saved["name"] != "" {
		t.Fatalf("profile did not survive a new session: %+v", saved)
	}
}
