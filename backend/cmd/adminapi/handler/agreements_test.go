package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestAgreementEditorPublishesCompatibleVersions(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	bootstrapTestAdmin(t, service)
	login, err := service.LoginAdmin(context.Background(), testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := adminhandler.NewRouter(service)
	request := func(method, payload string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/agreements", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+login.Token)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s returned HTTP %d, want %d", method, res.Code, want)
		}

		return res
	}

	request(http.MethodPost, `{"title":"旧版","body":"第一行\n<原文>"}`, http.StatusOK)
	request(http.MethodPost, `{"title":"新版","body_html":"<h2>条款</h2><p><u>重点</u></p>"}`, http.StatusOK)
	request(http.MethodPost, `{"title":"空正文","body_html":"<p><br></p>"}`, http.StatusBadRequest)
	res := request(http.MethodGet, "", http.StatusOK)
	var result struct {
		Data []agreement.DTO `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || len(result.Data) != 2 {
		t.Fatalf("agreement list: got %d versions, err=%v", len(result.Data), err)
	}

	current, old := result.Data[0], result.Data[1]
	if !current.Active || current.Body != "条款\n重点" || current.BodyHTML != "<h2>条款</h2><p><ins>重点</ins></p>" {
		t.Fatal("rich agreement did not preserve its formatting and plain fallback")
	}

	if old.Active || old.Body != "第一行\n<原文>" || old.BodyHTML != "" {
		t.Fatal("old agreement changed after publishing rich text")
	}
}
