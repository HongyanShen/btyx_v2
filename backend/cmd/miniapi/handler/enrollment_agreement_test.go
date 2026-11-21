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
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestEnrollmentConfirmsCurrentAgreement(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	login, err := services.DemoLogin(ctx, "enrollment-agreement")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.Client.UserVerification.Create().
		SetUserID(login.UserID).
		SetVerifiedAt(services.Now()).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	router := minihandler.NewRouter(services)
	request := func(method, route, body string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1"+route, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+login.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != wantStatus {
			t.Fatalf("%s %s got %d, want %d", method, route, response.Code, wantStatus)
		}

		return response
	}

	readAgreement := func() string {
		t.Helper()
		response := request(http.MethodGet, "/agreement", "", http.StatusOK)
		var envelope struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Data.ID == "" {
			t.Fatal("no agreement version returned")
		}

		return envelope.Data.ID
	}

	const fields = `"douyin":"dy_2026","xhs":"xhs_2026"`
	apply := func(agreementID string, wantStatus int) {
		t.Helper()
		request(http.MethodPost, "/enrollment", "{"+fields+`,"agreement_id":"`+agreementID+`"}`, wantStatus)
	}

	request(http.MethodPost, "/enrollment", "{"+fields+"}", http.StatusBadRequest)
	request(http.MethodGet, "/agreement", "", http.StatusConflict)
	if err := services.PublishAgreement(ctx, "admin", "营销分发任务协议", "首版报名协议。"); err != nil {
		t.Fatal(err)
	}

	old := readAgreement()
	if err := services.PublishAgreement(ctx, "admin", "新版营销分发任务协议", "本版本由报名人提交前确认。"); err != nil {
		t.Fatal(err)
	}

	apply(old, http.StatusConflict)
	profile, err := services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentNone || profile.Consented {
		t.Fatal("an unsigned or stale submission changed enrollment or consent")
	}

	current := readAgreement()
	_, err = services.DB.ExecContext(ctx, `
		ALTER TABLE user_enrollments ADD CHECK (douyin <> 'failed-application')
	`)
	if err != nil {
		t.Fatal(err)
	}

	request(http.MethodPost, "/enrollment", `{"douyin":"failed-application","xhs":"xhs_2026","agreement_id":"`+current+`"}`, http.StatusConflict)
	consents, err := services.Client.Consent.Query().Count(ctx)
	if err != nil || consents != 0 {
		t.Fatalf("failed enrollment kept a consent: count=%d, err=%v", consents, err)
	}

	apply(current, http.StatusOK)
	apply(current, http.StatusOK)
	profile, err = services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentPending || !profile.Consented {
		t.Fatal("a successful enrollment did not record the agreement")
	}

	consent, err := services.Client.Consent.Query().Only(ctx)
	if err != nil || consent.AgreementID != current || consent.UserID != login.UserID {
		t.Fatal("the exact accepted agreement must be stored once")
	}

	count, err := services.Client.AuditEvent.Query().
		Where(auditevent.ActionEQ(enum.AuditAgreementConsent)).
		Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("agreement consent audit count = %d, err = %v", count, err)
	}

	if err := services.PublishAgreement(ctx, "admin", "下一版营销分发任务协议", "新版本须再次阅读。"); err != nil {
		t.Fatal(err)
	}

	apply(current, http.StatusOK)
	apply(readAgreement(), http.StatusConflict)
	profile, err = services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentPending || profile.Consented {
		t.Fatal("a repeated application unexpectedly agreed to a newer version")
	}
}
