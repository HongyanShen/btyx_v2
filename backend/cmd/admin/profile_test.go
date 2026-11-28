package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestOperatorPersonalProfile(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := t.Context()
	root, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	request := func(token, method, path string, fields url.Values, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(fields.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if token != "" {
			setAdminSession(req, token)
		}

		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		if result.Code != status {
			t.Fatalf("%s %s status %d, want %d", method, path, result.Code, status)
		}

		return result.Body.String()
	}
	request("", "GET", "/profile", nil, 303)
	request(root.Token, "GET", "/profile", nil, 200)
	_, err = service.CreateOperator(ctx, root.UserID, identity.CreateOperatorInput{
		Username: "publisher",
		Password: testAdminPassword,
		OperatorGrants: identity.OperatorGrants{
			Pages:     []string{},
			Endpoints: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	login, err := service.LoginAdmin(ctx, "publisher", testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PublishAgreement(ctx, login.UserID, "发布人测试", "测试正文"); err != nil {
		t.Fatal(err)
	}

	if body := request(root.Token, "GET", "/agreements", nil, 200); !strings.Contains(body, ">publisher</td>") {
		t.Fatal("publisher must show the username")
	}

	if body := request(login.Token, "GET", "/profile", nil, 200); strings.Contains(body, "display_name") || !strings.Contains(body, "重置密码") {
		t.Fatal("personal information should only offer a password reset")
	}

	if body := request(root.Token, "GET", "/audits", nil, 200); !strings.Contains(body, ">publisher</td>") {
		t.Fatal("audit must show the operator username")
	}

	request(login.Token, "GET", "/operators", nil, 403)
	form := url.Values{
		"current_password": {"wrong-password"},
		"new_password":     {"profile-new-test-password"},
		"confirm_password": {"profile-new-test-password"},
	}
	request(login.Token, "POST", "/profile/password", form, 403)
	form.Set("csrf", csrfToken(login.Token))
	request(login.Token, "POST", "/profile/password", form, 400)
	secondSession, err := service.LoginAdmin(ctx, "publisher", testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	form.Set("current_password", testAdminPassword)
	request(login.Token, "POST", "/profile/password", form, 303)
	request(login.Token, "GET", "/profile", nil, 303)
	request(secondSession.Token, "GET", "/profile", nil, 303)
	if _, err := service.LoginAdmin(ctx, "publisher", testAdminPassword); err == nil {
		t.Fatal("old password still accepted")
	}

	if _, err := service.LoginAdmin(ctx, "publisher", form.Get("new_password")); err != nil {
		t.Fatal(err)
	}

	request(root.Token, "GET", "/profile", nil, 200)
}
