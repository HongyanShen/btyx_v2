package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestDistributorManagement(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	admin, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.DemoLogin(ctx, "distributor-management")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentApproved).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := service.PublishAgreement(ctx, admin.UserID, "测试协议", "测试营销分发协议"); err != nil {
		t.Fatal(err)
	}

	if err := service.Consent(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	encrypt := func(value string) string {
		t.Helper()
		cipher, err := service.Encrypt(value)
		if err != nil {
			t.Fatal(err)
		}

		return cipher
	}

	if err := service.Client.UserVerification.Create().
		SetUserID(member.UserID).
		SetNameCipher(encrypt("测试分发员")).
		SetContactPhoneCipher(encrypt("13800001111")).
		SetWechatCipher(encrypt("test-wechat")).
		SetIdentityCipher(encrypt("990000199001010017")).
		SetVerifiedAt(service.Now()).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	// The existing general user index limits its results to the newest 200 users.
	builders := make([]*ent.UserCreate, 0, 201)
	for range 201 {
		builders = append(builders, service.Client.User.Create())
	}

	if err := service.Client.User.CreateBulk(builders...).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	createTask := func(title string) string {
		t.Helper()
		version, err := service.CreateTask(ctx, admin.UserID, task.TaskInput{
			Title:     title,
			Body:      "测试发布要求",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin},
		})
		if err != nil {
			t.Fatal(err)
		}

		return version.TaskID
	}

	assignment, err := service.Assign(ctx, admin.UserID, member.UserID, createTask("今日指派"), "distributor-assignment")
	if err != nil {
		t.Fatal(err)
	}

	router := adminhandler.NewRouter(service)
	requestWithToken := func(method, path, body, token string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s: got %d, want %d", method, path, response.Code, status)
		}

		return response.Body.String()
	}

	request := func(method, path, body string, status int) string {
		t.Helper()
		return requestWithToken(method, path, body, admin.Token, status)
	}

	listPath := "/users?view=distributors"
	body := request("GET", listPath, "", 200)
	var list struct {
		Data []identity.UserDTO `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}

	if len(list.Data) != 1 || list.Data[0].ID != member.UserID ||
		list.Data[0].Wechat != "test-wechat" || list.Data[0].Phone != "138****1111" {
		t.Fatal("distributor list lost approved members, contact account or masked phone")
	}

	detailPath := listPath + "&id=" + member.UserID
	detail := request("GET", detailPath, "", 200)
	for _, value := range []string{"测试分发员", "13800001111", "9900**********0017", "今日指派", "assigned", "in_progress"} {
		if !strings.Contains(detail, value) {
			t.Errorf("distributor detail is missing %q", value)
		}
	}

	if strings.Contains(detail, "990000199001010017") {
		t.Error("distributor detail exposed an unmasked identity number")
	}

	var initial struct {
		Data []identity.UserDTO `json:"data"`
	}
	if err := json.Unmarshal([]byte(detail), &initial); err != nil || len(initial.Data) != 1 || initial.Data[0].DistributorDetails == nil {
		t.Fatal("expected distributor detail with a revision")
	}

	request("POST", "/operators", `{"username":"distributor-reader","password":"test-reader-password","pages":["/users"],"endpoints":["GET /users"]}`, 200)
	reader, err := service.LoginAdmin(ctx, "distributor-reader", "test-reader-password")
	if err != nil {
		t.Fatal(err)
	}

	masked := requestWithToken("GET", detailPath, "", reader.Token, 200)
	if strings.Contains(masked, "13800001111") || !strings.Contains(masked, "138****1111") {
		t.Fatal("read-only operator did not receive a masked contact phone")
	}

	blockPath := "/users/" + member.UserID + "/block"
	requestWithToken("POST", blockPath, `{"blocked":true,"reason":"无操作权限"}`, reader.Token, 403)
	request("POST", blockPath, `{"reason":"缺少目标状态"}`, 400)
	request("POST", blockPath, `{"blocked":true,"reason":"   "}`, 400)
	blockBody, err := json.Marshal(struct {
		Blocked   bool      `json:"blocked"`
		Reason    string    `json:"reason"`
		UpdatedAt time.Time `json:"updated_at"`
	}{
		Blocked:   true,
		Reason:    "  多次删除发布作品  ",
		UpdatedAt: initial.Data[0].DistributorDetails.UpdatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	request("POST", blockPath, string(blockBody), 200)
	request("POST", blockPath, string(blockBody), 200)
	detail = request("GET", detailPath, "", 200)
	for _, value := range []string{`"blocked":true`, "多次删除发布作品", testAdminUsername, "今日指派"} {
		if !strings.Contains(detail, value) {
			t.Errorf("blocked distributor detail is missing %q", value)
		}
	}

	profile, err := service.User(ctx, member.UserID)
	if err != nil || profile.BlockReason != "多次删除发布作品" {
		t.Fatal("blocked distributor cannot see the reason")
	}

	unallocated := createTask("未指派内容")
	assertForbidden := func(err error) {
		t.Helper()
		var failure *app.Error
		if !errors.As(err, &failure) || failure.Status != 403 {
			t.Fatalf("expected blocked allocation, got %v", err)
		}
	}

	_, err = service.Claim(ctx, member.UserID, unallocated, "blocked-claim")
	assertForbidden(err)
	_, err = service.Assign(ctx, admin.UserID, member.UserID, unallocated, "blocked-assignment")
	assertForbidden(err)

	if err := service.Submit(ctx, member.UserID, assignment.ID, submission.SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/distributor-work/",
	}); err != nil {
		t.Fatal(err)
	}

	if err := service.ReviewAtRevision(ctx, admin.UserID, assignment.ID, submission.ReviewInput{
		Platform: enum.PlatformDouyin,
		Revision: 1,
		Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = service.Reissue(ctx, admin.UserID, assignment.ID, enum.PlatformDouyin, "测试重派", false)
	assertForbidden(err)
	detail = request("GET", detailPath, "", 200)
	if !strings.Contains(detail, `"status":"pending_settlement"`) || strings.Contains(detail, `"unpublished":true`) {
		t.Fatal("existing published task did not keep its settlement status")
	}

	request("POST", blockPath, `{"blocked":false}`, 200)
	request("POST", blockPath, `{"blocked":false}`, 200)
	request("POST", blockPath, string(blockBody), 409)
	detail = request("GET", detailPath, "", 200)
	if !strings.Contains(detail, `"blocked":false`) || strings.Contains(detail, "多次删除发布作品") {
		t.Fatal("lifting the block did not clear the current block information")
	}

	events, err := service.Client.AuditEvent.Query().
		Where(auditevent.ReferenceIDEQ(member.UserID), auditevent.ActionEQ(enum.AuditUserBlock)).
		Count(ctx)
	if err != nil || events != 2 {
		t.Fatalf("expected one audit per status transition, got %d: %v", events, err)
	}
}
