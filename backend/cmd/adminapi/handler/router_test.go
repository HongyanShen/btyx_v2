package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestAPIsEnforceAuthenticationAndProcessBoundary(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	admin := adminhandler.NewRouter(service)
	mini := minihandler.NewRouter(service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	response := httptest.NewRecorder()
	admin.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous admin access: %d", response.Code)
	}

	login, err := service.DemoLogin(context.Background(), "http-boundary")
	if err != nil {
		t.Fatal(err)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	request.Header.Set("Authorization", "Bearer "+login.Token)
	response = httptest.NewRecorder()
	admin.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("user reached admin API: %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+login.Token)
	response = httptest.NewRecorder()
	mini.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("user profile failed: %d", response.Code)
	}

	if strings.Contains(response.Body.String(), "openid") {
		t.Fatal("private provider identity exposed through public DTO")
	}
}

func TestAPIRejectsUnknownPlatformAndReturnsStandardEnvelope(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	router := adminhandler.NewRouter(service)
	bootstrapTestAdmin(t, service)
	login, err := service.LoginAdmin(context.Background(), testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tasks",
		strings.NewReader(`{"title":"测试","body":"发布要求","price_fen":"750","platforms":["unknown"]}`),
	)
	request.Header.Set("Authorization", "Bearer "+login.Token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid enum accepted: %d, %s", response.Code, response.Body.String())
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}

	if envelope.Code == 0 || envelope.Msg == "" || string(envelope.Data) != "null" {
		t.Fatalf("invalid error envelope: %s", response.Body.String())
	}
}
