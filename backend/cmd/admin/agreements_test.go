package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestAgreementEditorPublishesCompatibleVersions(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	login, err := service.LoginAdmin(context.Background(), testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	request := func(method, payload string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/agreements", strings.NewReader(payload))
		setAdminSession(req, login.Token)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s returned HTTP %d, want %d", method, res.Code, want)
		}

		return res
	}

	request(http.MethodPost, `{"title":"旧版","body":"第一行\n<strong>原文</strong>"}`, http.StatusOK)
	request(http.MethodPost, `{"title":"新版","body_html":"<h2>条款</h2><ol><li><strong>正文</strong><u>重点</u></li></ol>"}`, http.StatusOK)
	request(http.MethodPost, `{"title":"含图片","body_html":"<p>正文</p><img src=\"image.png\">"}`, http.StatusBadRequest)
	request(http.MethodPost, `{"title":"空正文","body_html":"<p><br></p>"}`, http.StatusBadRequest)
	res := request(http.MethodGet, "", http.StatusOK)
	var result struct {
		Data []agreement.DTO `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || len(result.Data) != 2 {
		t.Fatalf("agreement list: got %d versions, err=%v", len(result.Data), err)
	}

	current, old := result.Data[0], result.Data[1]
	if !current.Active || current.Body != "条款\n1. 正文重点" || current.BodyHTML != "<h2>条款</h2><ol><li><strong>正文</strong><ins>重点</ins></li></ol>" {
		t.Fatalf("unexpected agreement markup %q or plain text %q", current.BodyHTML, current.Body)
	}

	if old.Active || old.Body != "第一行\n<strong>原文</strong>" || old.BodyHTML != "" {
		t.Fatal("old agreement changed after publishing rich text")
	}

	page := httptest.NewRequest(http.MethodGet, "/agreements", nil)
	setAdminSession(page, login.Token)
	html := httptest.NewRecorder()
	router.ServeHTTP(html, page)
	if html.Code != http.StatusOK || !strings.Contains(html.Body.String(), "第一行\n&lt;strong&gt;原文&lt;/strong&gt;") ||
		!strings.Contains(html.Body.String(), current.BodyHTML) || strings.Contains(html.Body.String(), "image.png") {
		t.Fatal("agreement page did not render legacy text and sanitized markup")
	}
}
