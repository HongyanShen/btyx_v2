package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

const (
	testAdminUsername = "test-admin"
	testAdminPassword = "test-operator-password"
)

func bootstrapTestAdmin(t *testing.T, service *business.Service) {
	t.Helper()

	if err := service.BootstrapAdmin(context.Background(), testAdminUsername, testAdminPassword); err != nil {
		t.Fatal(err)
	}
}

func TestOperatorAccessUsesCurrentAccountGrants(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	bootstrapTestAdmin(t, service)
	router := adminhandler.NewRouter(service)

	request := func(method, path, token, body string, want int) json.RawMessage {
		t.Helper()

		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.Code, want, response.Body.String())
		}

		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}

		return envelope.Data
	}

	login := func(body string) identity.LoginResult {
		t.Helper()

		var result identity.LoginResult
		if err := json.Unmarshal(request("POST", "/auth/login", "", body, 200), &result); err != nil {
			t.Fatal(err)
		}

		return result
	}

	request("POST", "/auth/login", "", `{"password":"test-operator-password"}`, 400)
	root := login(`{"username":"test-admin","password":"test-operator-password"}`)
	request("POST", "/auth/login", "", `{"username":"test-admin","password":"wrong-password"}`, 401)
	request("POST", "/operators", root.Token, `{"username":"reviewer","password":"test-reviewer-password","pages":["/tasks"],"endpoints":["GET /tasks","POST /unregistered"]}`, 400)
	created := request("POST", "/operators", root.Token, `{"username":"reviewer","password":"test-reviewer-password","pages":["/tasks"],"endpoints":["GET /tasks","GET /feedback"]}`, 200)
	var account struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created, &account); err != nil || account.ID == "" {
		t.Fatalf("missing operator ID: %s", created)
	}

	operator := login(`{"username":"reviewer","password":"test-reviewer-password"}`)
	if operator.UserID != account.ID || operator.UserID == root.UserID {
		t.Fatalf("operator accounts must have distinct IDs: %q, %q", root.UserID, operator.UserID)
	}

	profile := request("GET", "/auth/me", operator.Token, "", 200)
	if !strings.Contains(string(profile), `"pages":["/tasks"]`) || strings.Contains(string(profile), "password") {
		t.Fatalf("unexpected current grants: %s", profile)
	}

	request("GET", "/tasks", operator.Token, "", 200)
	request("POST", "/tasks", operator.Token, `{}`, 403)
	request("GET", "/marketing-settings", "", "", 401)
	request("GET", "/marketing-settings", operator.Token, "", 403)
	settings := request("GET", "/marketing-settings", root.Token, "", 200)
	if !strings.Contains(string(settings), `"expiry_minutes":60`) {
		t.Fatalf("unexpected marketing settings: %s", settings)
	}

	request("POST", "/marketing-settings", operator.Token, `{"expiry_minutes":90}`, 403)
	updatedSettings := request("POST", "/marketing-settings", root.Token, `{"expiry_minutes":90}`, 200)
	readSettings := request("GET", "/marketing-settings", root.Token, "", 200)
	if !strings.Contains(string(updatedSettings), `"expiry_minutes":90`) ||
		!strings.Contains(string(readSettings), `"expiry_minutes":90`) {
		t.Fatalf("updated expiry was not saved: POST %s, GET %s", updatedSettings, readSettings)
	}

	request("POST", "/marketing-settings", root.Token, `{"expiry_mode":"invalid"}`, 400)
	request("POST", "/marketing-settings", root.Token, `{"expiry_mode":"fixed_duration","expiry_minutes":1470}`, 400)
	request("POST", "/marketing-settings", root.Token, `{"expiry_mode":"end_of_day","expiry_minutes":60}`, 400)
	request("POST", "/marketing-settings", root.Token, `{"expiry_mode":"end_of_day"}`, 200)
	daySettings := request("GET", "/marketing-settings", root.Token, "", 200)
	if !strings.Contains(string(daySettings), `"expiry_mode":"end_of_day"`) ||
		!strings.Contains(string(daySettings), `"expiry_minutes":90`) {
		t.Fatalf("day mode or retained duration missing: %s", daySettings)
	}

	request("GET", "/users", operator.Token, "", 403)
	request("GET", "/files/example/download", operator.Token, "", 403)
	request("GET", "/operators", operator.Token, "", 403)
	request("GET", "/operators/permissions", root.Token, "", 200)
	request("POST", "/operators", root.Token, `{"username":"REVIEWER","password":"test-reviewer-password","pages":[],"endpoints":[]}`, 409)
	request("POST", "/operators/"+root.UserID, root.Token, `{"pages":[],"endpoints":[],"disabled":true}`, 403)

	ctx := context.Background()
	if err := service.BootstrapAdmin(ctx, "another-admin", testAdminPassword); err == nil {
		t.Fatal("a second super administrator was bootstrapped")
	}

	distributor, err := service.DemoLogin(ctx, "operator-permission-feedback")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.CreateFeedback(ctx, distributor.UserID, "测试反馈"); err != nil {
		t.Fatal(err)
	}

	feedback := request("GET", "/feedback", operator.Token, "", 200)
	if !strings.Contains(string(feedback), distributor.UserID) {
		t.Fatalf("operator with feedback access did not see distributor feedback: %s", feedback)
	}

	miniRequest := httptest.NewRequest("GET", "/api/v1/me", nil)
	miniRequest.Header.Set("Authorization", "Bearer "+operator.Token)
	miniResponse := httptest.NewRecorder()
	minihandler.NewRouter(service).ServeHTTP(miniResponse, miniRequest)
	if miniResponse.Code != 403 {
		t.Fatalf("mini API accepted an operator account: %d", miniResponse.Code)
	}

	_, err = service.Client.Session.Create().
		SetUserID("admin").
		SetTokenHash(app.Hash("legacy-shared-session")).
		SetRole(enum.RoleAdmin).
		SetExpiresAt(service.Now().Add(time.Hour)).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	request("GET", "/auth/me", "legacy-shared-session", "", 401)

	request("POST", "/operators/"+account.ID, root.Token, `{"pages":[],"endpoints":[],"disabled":false}`, 200)
	request("GET", "/tasks", operator.Token, "", 403)
	request("POST", "/operators/"+account.ID, root.Token, `{"pages":[],"endpoints":[],"disabled":true}`, 200)
	request("GET", "/auth/me", operator.Token, "", 401)
	request("POST", "/operators/"+account.ID, root.Token, `{"pages":[],"endpoints":["GET /tasks"],"disabled":false}`, 200)
	request("GET", "/tasks", operator.Token, "", 401)

	operator = login(`{"username":"reviewer","password":"test-reviewer-password"}`)
	request("POST", "/operators/"+account.ID, root.Token, `{"password":"test-updated-password","pages":[],"endpoints":["POST /tasks"],"disabled":false}`, 200)
	request("GET", "/auth/me", operator.Token, "", 401)
	request("POST", "/auth/login", "", `{"username":"reviewer","password":"test-reviewer-password"}`, 401)
	operator = login(`{"username":"reviewer","password":"test-updated-password"}`)
	taskWithFile := func(ownerID string) string {
		t.Helper()

		record, err := service.Client.FileObject.Create().
			SetOwnerID(ownerID).
			SetName("approved.png").
			SetMediaType(enum.FilePNG).
			SetSize(12).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return fmt.Sprintf(`{"title":"授权测试","body":"发布要求","price_fen":"750","platforms":["dy"],"files":[%q]}`, record.ID)
	}

	request("POST", "/tasks", operator.Token, taskWithFile(distributor.UserID), 400)
	task := request("POST", "/tasks", operator.Token, taskWithFile(root.UserID), 200)
	if !strings.Contains(string(task), operator.UserID) {
		t.Fatalf("task version did not record the individual creator: %s", task)
	}

	audit := request("GET", "/audits", root.Token, "", 200)
	if !strings.Contains(string(audit), `"actor_username":"reviewer","action":"task.create"`) {
		t.Fatalf("operator mutation did not have an individual audit actor: %s", audit)
	}
}
