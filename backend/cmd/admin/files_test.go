package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestUploadWithoutTOSConfigurationCannotReportSuccess(t *testing.T) {
	services := testkit.Service(t, "../../..")
	services.FilesService.Store = nil
	bootstrapTestAdmin(t, services)
	login, err := services.LoginAdmin(context.Background(), testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, services)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/files/uploads", strings.NewReader(`{"name":"test.png","media_type":"image/png","size":8}`))
	setAdminSession(request, login.Token)
	request.Header.Set("Idempotency-Key", "missing-storage-config")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "TOS") {
		t.Fatalf("missing configuration became success: status=%d", response.Code)
	}

	count, err := services.Client.FileObject.Query().Count(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("created uploads without a configured store: count=%d, err=%v", count, err)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/files", strings.NewReader("file-content-must-not-pass-through-api"))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code >= 200 && response.Code < 300 {
		t.Fatal("legacy file-body upload endpoint is still enabled")
	}
}
