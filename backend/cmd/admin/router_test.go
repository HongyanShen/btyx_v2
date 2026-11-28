package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/gin-gonic/gin"
)

func TestAdminRendersPagesAndRepliesInPostgres(t *testing.T) {
	service := testkit.Service(t, "../../..")
	ctx := context.Background()
	if err := service.BootstrapAdmin(ctx, "htmx-test", "test-admin-password"); err != nil {
		t.Fatal(err)
	}

	user, err := service.DemoLogin(ctx, "htmx-feedback-test")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.CreateFeedback(ctx, user.UserID, "<script>测试反馈</script>"); err != nil {
		t.Fatal(err)
	}

	feedbacks, err := service.Feedbacks(ctx, user.UserID, user.Role)
	if err != nil || len(feedbacks) != 1 {
		t.Fatalf("feedback fixture: %v, rows=%d", err, len(feedbacks))
	}

	router := testAdminRouter(t, service)

	var session *http.Cookie
	request := func(method, path, body string, headers map[string]string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://admin.example.test"+path, strings.NewReader(body))
		if session != nil {
			req.AddCookie(session)
		}

		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		for name, value := range headers {
			req.Header.Set(name, value)
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: status %d, want %d", method, path, response.Code, want)
		}

		return response
	}

	request("GET", "/reviews", "", nil, http.StatusSeeOther)
	request("GET", "/api/v1/auth/me", "", nil, http.StatusUnauthorized)
	request("POST", "/api/v1/auth/login", "", nil, http.StatusNotFound)
	request("GET", "/workspace.html", "", nil, http.StatusNotFound)
	loginPage := request("GET", "/login", "", nil, http.StatusOK)
	if !strings.Contains(loginPage.Body.String(), `type="module" src="/assets/app/workspace.js"`) {
		t.Fatal("login must load the embedded browser entry point")
	}

	for _, asset := range []struct {
		path      string
		mediaType string
	}{
		{"/assets/app/workspace.js", "javascript"},
		{"/assets/modules/support/agreement-form.js", "javascript"},
		{"/assets/styles/global.css", "text/css"},
		{"/assets/vendor/htmx.js", "javascript"},
		{"/assets/vendor/axios.js", "javascript"},
		{"/assets/vendor/purify.js", "javascript"},
		{"/assets/vendor/quill.js", "javascript"},
		{"/assets/vendor/quill.snow.css", "text/css"},
	} {
		response := request("GET", asset.path, "", nil, http.StatusOK)
		if response.Body.Len() == 0 || !strings.Contains(response.Header().Get("Content-Type"), asset.mediaType) {
			t.Errorf("%s must serve a nonempty %s resource", asset.path, asset.mediaType)
		}
	}

	login := "username=htmx-test&password=test-admin-password&next=%2Ffeedback"
	request("POST", "/login", login, map[string]string{"Origin": "https://other.example.test"}, http.StatusForbidden)
	response := request("POST", "/login", login, nil, http.StatusSeeOther)
	if response.Header().Get("Location") != "/feedback" {
		t.Fatal("login did not preserve the current page")
	}

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookie {
			session = cookie
		}
	}

	if session == nil || !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteStrictMode {
		t.Fatal("admin session cookie lacks required attributes")
	}

	response = request("GET", "/feedback", "", nil, http.StatusOK)
	page := response.Body.String()
	if !strings.Contains(page, "<!doctype html>") || !strings.Contains(page, "htmx-test") ||
		!strings.Contains(page, "&lt;script&gt;测试反馈&lt;/script&gt;") || strings.Contains(page, session.Value) ||
		!strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("feedback page must render escaped data and avoid caching or exposing the session")
	}

	fragment := request("GET", "/reviews", "", map[string]string{"HX-Request": "true"}, http.StatusOK).Body.String()
	if strings.Contains(fragment, "<!doctype html>") || strings.Contains(fragment, `data-workspace-page`) ||
		!strings.Contains(fragment, `aria-label="发布链接审核"`) {
		t.Fatal("htmx navigation did not return the requested workspace fragment")
	}

	for _, view := range []struct {
		path, heading, control string
	}{
		{
			path:    "/ledger",
			heading: "余额流水",
			control: "可用余额变动",
		},
		{
			path:    "/agreements",
			heading: "分发协议",
			control: "发布新协议",
		},
		{
			path:    "/operators",
			heading: "运营账号",
			control: "新建运营账号",
		},
		{
			path:    "/contents",
			heading: "素材库",
			control: "新增内容",
		},
		{
			path:    "/users/manage",
			heading: "分发员管理",
			control: "搜索分发员",
		},
		{
			path:    "/users",
			heading: "分发员报名审核",
			control: "人工复核",
		},
		{
			path:    "/settlements",
			heading: "分发结算",
			control: "结算月份",
		},
		{
			path:    "/distribution/settings",
			heading: "营销设置",
			control: "保存分发设置",
		},
		{
			path:    "/tasks",
			heading: "任务管理",
			control: "上传营销内容",
		},
	} {
		body := request("GET", view.path, "", nil, http.StatusOK).Body.String()
		if strings.Contains(body, "data-workspace-page") || !strings.Contains(body, view.heading) ||
			!strings.Contains(body, view.control) {
			t.Fatalf("%s must return its business page as HTML", view.path)
		}
	}

	request("GET", "/api/v1/auth/me", "", nil, http.StatusOK)
	token := session.Value
	session = nil
	request("GET", "/api/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + token}, http.StatusUnauthorized)
	session = &http.Cookie{
		Name:  sessionCookie,
		Value: token,
	}
	request("POST", "/api/v1/marketing-settings", `{}`, map[string]string{"Content-Type": "application/json"}, http.StatusForbidden)
	replyPath := "/feedback/" + feedbacks[0].ID + "/reply"
	form := url.Values{
		"reply": {"已核实"},
		"csrf":  {csrfToken(session.Value)},
	}
	request("POST", replyPath, "reply=invalid", nil, http.StatusForbidden)
	request("POST", replyPath, form.Encode(), map[string]string{"HX-Request": "true"}, http.StatusOK)
	feedbacks, err = service.Feedbacks(ctx, user.UserID, user.Role)
	if err != nil || feedbacks[0].Reply != "已核实" {
		t.Fatalf("reply was not saved: %v", err)
	}

	root, err := service.Authenticate(ctx, session.Value)
	if err != nil {
		t.Fatal(err)
	}

	request("POST", "/logout", form.Encode(), nil, http.StatusSeeOther)
	request("GET", "/api/v1/auth/me", "", nil, http.StatusUnauthorized)

	grants := identity.OperatorGrants{
		Pages:     []string{"/audits"},
		Endpoints: []string{"GET /audits"},
	}
	account, err := service.CreateOperator(ctx, root.ID, identity.CreateOperatorInput{
		Username:       "limited-admin",
		Password:       "limited-admin-password",
		OperatorGrants: grants,
	})
	if err != nil {
		t.Fatal(err)
	}

	response = request("POST", "/login", "username=limited-admin&password=limited-admin-password", nil, http.StatusSeeOther)
	session = response.Result().Cookies()[0]
	request("GET", "/audits", "", nil, http.StatusOK)
	request("GET", "/feedback", "", nil, http.StatusForbidden)
	enabled := false
	for _, test := range []struct {
		pages     bool
		endpoints bool
		want      int
	}{
		{true, false, http.StatusForbidden},
		{false, true, http.StatusForbidden},
		{true, true, http.StatusOK},
		{false, false, http.StatusForbidden},
	} {
		grants.AllPages = test.pages
		grants.AllEndpoints = test.endpoints
		if _, err := service.UpdateOperator(ctx, root.ID, account.ID, identity.UpdateOperatorInput{
			Disabled:       &enabled,
			OperatorGrants: grants,
		}); err != nil {
			t.Fatal(err)
		}

		request("GET", "/feedback", "", nil, test.want)
		request("GET", "/operators", "", nil, http.StatusForbidden)
		request("GET", "/api/v1/operators/permissions", "", nil, http.StatusForbidden)
	}

	disabled := true
	if _, err := service.UpdateOperator(ctx, root.ID, account.ID, identity.UpdateOperatorInput{
		Disabled:       &disabled,
		OperatorGrants: grants,
	}); err != nil {
		t.Fatal(err)
	}

	request("GET", "/api/v1/auth/me", "", nil, http.StatusUnauthorized)
	response = request("GET", "/audits", "", map[string]string{"HX-Request": "true"}, http.StatusOK)
	if !strings.HasPrefix(response.Header().Get("HX-Redirect"), "/login?") {
		t.Fatal("rejected HTMX session must redirect to login")
	}
}

func testAdminRouter(t *testing.T, service *business.Service) *gin.Engine {
	t.Helper()

	router, err := newAdminRouter(service, true)
	if err != nil {
		t.Fatal(err)
	}

	return router
}

func setAdminSession(request *http.Request, token string) {
	request.AddCookie(&http.Cookie{
		Name:  sessionCookie,
		Value: token,
	})
	request.Header.Set("X-CSRF-Token", csrfToken(token))
}
