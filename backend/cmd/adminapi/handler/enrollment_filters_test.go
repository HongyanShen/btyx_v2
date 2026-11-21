package handler_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestEnrollmentFiltersApplyBeforePagination(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	admin, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := adminhandler.NewRouter(service)
	applications := []struct {
		name   string
		status enum.EnrollmentStatus
		tag    string
	}{
		{"pending-fashion", enum.EnrollmentPending, "穿搭"},
		{"pending-fitness", enum.EnrollmentPending, "健身"},
		{"approved-fitness", enum.EnrollmentApproved, "健身"},
	}
	for _, application := range applications {
		member, err := service.DemoLogin(ctx, application.name)
		if err != nil {
			t.Fatal(err)
		}

		err = service.Client.UserEnrollment.Create().
			SetUserID(member.UserID).
			SetStatus(application.status).
			SetTags([]string{application.tag}).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	for range 30 {
		member, err := service.Client.User.Create().Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = service.Client.UserEnrollment.Create().
			SetUserID(member.ID).
			SetStatus(enum.EnrollmentPending).
			SetTags([]string{"健身"}).
			Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	seen := make(map[string]bool)
	for _, test := range []struct {
		query  string
		status int
		total  string
		rows   int
	}{
		{"&status=pending&tag=" + url.QueryEscape("健身") + "&page=1", 200, "31", 30},
		{"&status=pending&tag=" + url.QueryEscape("健身") + "&page=2", 200, "31", 1},
		{"&status=pending&tag=" + url.QueryEscape("健身") + "&page=3", 200, "31", 0},
		{"&status=invalid&page=1", 400, "", 0},
	} {
		request := httptest.NewRequest("GET", "/api/v1/users?view=enrollments"+test.query, nil)
		request.Header.Set("Authorization", "Bearer "+admin.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("X-Total-Count") != test.total {
			t.Fatalf("query %s: status %d, count %q", test.query, response.Code, response.Header().Get("X-Total-Count"))
		}

		if test.status == 200 {
			var envelope struct {
				Data []identity.UserDTO `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}

			if len(envelope.Data) != test.rows {
				t.Fatalf("query %s: got %d rows", test.query, len(envelope.Data))
			}

			for _, record := range envelope.Data {
				if record.Status != enum.EnrollmentPending || len(record.Tags) != 1 || record.Tags[0] != "健身" {
					t.Fatal("result does not match both filters")
				}

				if seen[record.ID] {
					t.Fatal("applicant was repeated across pages")
				}

				seen[record.ID] = true
			}
		}
	}
}

func TestMarketingTagDeletionRequiresUnbinding(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	admin, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := adminhandler.NewRouter(service)
	request := func(method, path, body string, status int) string {
		t.Helper()

		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+admin.Token)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s: got %d, want %d", method, path, response.Code, status)
		}

		return response.Body.String()
	}

	tag, err := service.CreateMarketingTag(ctx, admin.UserID, "健身")
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.DemoLogin(ctx, "marketing-tag-delete")
	if err != nil {
		t.Fatal(err)
	}

	application, err := service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentRejected).
		SetReason("资料待补充").
		SetRecentPost(false).
		SetNewAccount(false).
		SetAgeGroup(enum.EnrollmentAge25To30).
		SetTags([]string{tag.Key}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	deletePath := "/marketing-tags/" + tag.ID + "/delete"
	deleteInput, err := json.Marshal(map[string]any{"updated_at": tag.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}

	if body := request("POST", deletePath, string(deleteInput), 409); !strings.Contains(body, "分发员报名") {
		t.Fatal("expected a conflict identifying the enrollment binding")
	}

	application, err = service.Client.UserEnrollment.Get(ctx, application.ID)
	if err != nil {
		t.Fatal(err)
	}

	review, err := json.Marshal(map[string]any{
		"enrollment_updated_at": application.UpdatedAt,
		"tags_only":             true,
		"tags":                  []string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	request("POST", "/users/"+member.UserID+"/review", string(review), 200)
	application, err = service.Client.UserEnrollment.Get(ctx, application.ID)
	if err != nil {
		t.Fatal(err)
	}

	if application.Status != enum.EnrollmentRejected || application.Reason != "资料待补充" || len(application.Tags) != 0 {
		t.Fatal("unbinding tags affected the original review decision")
	}

	request("POST", deletePath, string(deleteInput), 200)
	request("POST", deletePath, string(deleteInput), 200)
	if strings.Contains(request("GET", "/marketing-tags", "", 200), tag.ID) {
		t.Fatal("deleted tag remains in the library")
	}

	request("POST", "/contents", `{"title":"素材","body":"文案","tags":["健身"]}`, 409)
	createdTag := request("POST", "/marketing-tags", `{"name":"健身"}`, 200)
	var replacementEnvelope struct {
		Data enrollment.MarketingTagDTO `json:"data"`
	}
	if err := json.Unmarshal([]byte(createdTag), &replacementEnvelope); err != nil {
		t.Fatal(err)
	}

	replacement := replacementEnvelope.Data
	if replacement.ID == tag.ID || replacement.Key == tag.Key || replacement.Name != tag.Name {
		t.Fatalf("same-name addition must create a distinct tag: %v", replacement)
	}

	request("POST", deletePath, string(deleteInput), 200)
	if !strings.Contains(request("GET", "/marketing-tags", "", 200), replacement.ID) {
		t.Fatal("an old delete request affected the new tag")
	}

	request("POST", "/contents", `{"title":"素材","body":"文案","tags":["健身"]}`, 409)
	deletePath = "/marketing-tags/" + replacement.ID + "/delete"
	deleteInput, err = json.Marshal(map[string]any{"updated_at": replacement.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}

	tagKeys, err := json.Marshal([]string{replacement.Key})
	if err != nil {
		t.Fatal(err)
	}

	created := request("POST", "/contents", `{"title":"素材","body":"文案","tags":`+string(tagKeys)+`}`, 200)
	if body := request("POST", deletePath, string(deleteInput), 409); !strings.Contains(body, "内容池") {
		t.Fatal("expected a conflict identifying the content binding")
	}

	var material struct {
		Data content.DTO `json:"data"`
	}
	if err := json.Unmarshal([]byte(created), &material); err != nil {
		t.Fatal(err)
	}

	request("POST", "/contents/"+material.Data.ID+"/tags", `{"previous_tags":`+string(tagKeys)+`,"tags":[]}`, 200)
	task := request("POST", "/tasks", `{
		"title":"测试任务","body":"发布要求","price_fen":"750","platforms":["dy"],"tags":`+string(tagKeys)+`
	}`, 200)
	if body := request("POST", deletePath, string(deleteInput), 409); !strings.Contains(body, "任务当前版本") {
		t.Fatal("expected a conflict identifying the active task version")
	}

	var published struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(task), &published); err != nil {
		t.Fatal(err)
	}

	request("POST", "/tasks/"+published.Data.TaskID+"/versions", `{
		"title":"测试任务","body":"发布要求","price_fen":"750","platforms":["dy"],"tags":[]
	}`, 200)
	request("POST", deletePath, string(deleteInput), 200)
}
