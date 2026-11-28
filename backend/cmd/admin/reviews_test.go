package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func newReviewAssignment(t *testing.T) (*business.Service, identity.LoginResult, task.AssignmentDTO) {
	t.Helper()
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	operator, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.DemoLogin(ctx, "review-list-member")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentApproved).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := service.PublishAgreement(ctx, operator.UserID, "测试协议", "测试营销分发协议"); err != nil {
		t.Fatal(err)
	}

	if err := service.Consent(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	version, err := service.CreateTask(ctx, operator.UserID, task.TaskInput{
		Title:    "发布链接审核验收",
		Body:     "发布作品",
		PriceFen: "750",
		Platforms: []enum.Platform{
			enum.PlatformDouyin,
			enum.PlatformXiaohongshu,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assignment, err := service.Claim(ctx, member.UserID, version.TaskID, "review-list")
	if err != nil {
		t.Fatal(err)
	}

	return service, operator, assignment
}

func TestReviewListUsesFilteredTotalsAndSubmissionTimes(t *testing.T) {
	service, operator, assignment := newReviewAssignment(t)
	ctx := context.Background()
	for _, url := range []string{"https://xhslink.com/a/first", "https://xhslink.com/a/second"} {
		if err := service.Submit(ctx, assignment.UserID, assignment.ID, submission.SubmitInput{
			Platform: enum.PlatformXiaohongshu,
			URL:      url,
		}); err != nil {
			t.Fatal(err)
		}
	}

	latest, err := service.Client.AuditEvent.Query().
		Where(
			auditevent.ActionEQ(enum.AuditSubmissionSubmit),
			auditevent.ReferenceIDEQ(assignment.ID),
		).
		Order(ent.Desc(auditevent.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	request := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assignments"+query, nil)
		setAdminSession(req, operator.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	for _, test := range []struct {
		query string
		count string
		rows  int
	}{
		{
			query: "?page=1&platform=xhs",
			count: "1",
			rows:  1,
		},
		{
			query: "?page=2&platform=xhs",
			count: "1",
		},
		{
			query: "?page=1&platform=dy",
			count: "0",
		},
		{
			query: "?page=1&status=settled",
			count: "0",
		},
	} {
		response := request(test.query)
		if response.Code != http.StatusOK || response.Header().Get("X-Total-Count") != test.count {
			t.Fatalf("%s returned %d, total %q", test.query, response.Code, response.Header().Get("X-Total-Count"))
		}

		var result struct {
			Data []struct {
				ID          string `json:"id"`
				Submissions []struct {
					Revision    int       `json:"revision"`
					SubmittedAt time.Time `json:"submitted_at"`
				} `json:"submissions"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}

		if len(result.Data) != test.rows {
			t.Fatalf("%s returned %d rows, want %d", test.query, len(result.Data), test.rows)
		}

		if test.rows == 1 {
			row := result.Data[0]
			if row.ID != assignment.ID || len(row.Submissions) != 1 || row.Submissions[0].Revision != 2 || !row.Submissions[0].SubmittedAt.Equal(latest.CreatedAt) {
				t.Fatalf("missing current submission and time: %+v", row)
			}
		}
	}

	for _, query := range []string{"?page=0", "?page=1&platform=other", "?page=1&status=other"} {
		if response := request(query); response.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d, want 400", query, response.Code)
		}
	}

	if err := service.Submit(ctx, assignment.UserID, assignment.ID, submission.SubmitInput{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/review-page/",
	}); err != nil {
		t.Fatal(err)
	}

	if err := service.ReviewAtRevision(ctx, operator.UserID, assignment.ID, submission.ReviewInput{
		Platform: enum.PlatformXiaohongshu,
		Revision: 2,
		Approved: false,
		Reason:   "请补齐必带话题",
	}); err != nil {
		t.Fatal(err)
	}

	for query, want := range map[string]string{
		"?page=1&status=pending_review":              "1",
		"?page=1&status=pending_review&platform=dy":  "1",
		"?page=1&status=pending_review&platform=xhs": "0",
		"?page=1&status=rejected&platform=dy":        "0",
		"?page=1&status=rejected&platform=xhs":       "1",
	} {
		response := request(query)
		if response.Code != http.StatusOK || response.Header().Get("X-Total-Count") != want {
			t.Fatalf("%s returned %d, total %q, want %s", query, response.Code, response.Header().Get("X-Total-Count"), want)
		}
	}
}

func TestMultiPlatformRejectionIsAtomicAndIdempotent(t *testing.T) {
	testMultiPlatformReview(t, false)
}

func TestMultiPlatformApprovalIsAtomicAndIdempotent(t *testing.T) {
	testMultiPlatformReview(t, true)
}

func testMultiPlatformReview(t *testing.T, approved bool) {
	t.Helper()
	service, operator, assignment := newReviewAssignment(t)
	ctx := context.Background()
	for platform, link := range map[enum.Platform]string{
		enum.PlatformDouyin:      "https://v.douyin.com/batch-review/",
		enum.PlatformXiaohongshu: "https://xhslink.com/a/batch-review",
	} {
		if err := service.Submit(ctx, assignment.UserID, assignment.ID, submission.SubmitInput{
			Platform: platform,
			URL:      link,
		}); err != nil {
			t.Fatal(err)
		}
	}

	router := testAdminRouter(t, service)
	requestReview := func(body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments/"+assignment.ID+"/review", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		setAdminSession(req, operator.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("review returned %d, want %d: %s", response.Code, want, response.Body.String())
		}
	}
	auditAction := enum.AuditSubmissionRejected
	if approved {
		auditAction = enum.AuditSubmissionApproved
	}

	assertReview := func(status enum.SubmissionStatus, assignmentStatus enum.AssignmentStatus, reason string, events int) {
		t.Helper()
		current, err := service.AdminAssignment(ctx, assignment.ID)
		if err != nil {
			t.Fatal(err)
		}

		if current.Status != assignmentStatus || len(current.Submissions) != 2 {
			t.Fatalf("unexpected assignment status %s or platform count %d", current.Status, len(current.Submissions))
		}

		for _, record := range current.Submissions {
			if record.Status != status || record.Reason != reason {
				t.Fatalf("unexpected review of %s: %s, %q", record.Platform, record.Status, record.Reason)
			}
		}

		audits, err := service.Client.AuditEvent.Query().
			Where(auditevent.ActionEQ(auditAction)).
			Count(ctx)
		if err != nil {
			t.Fatal(err)
		}

		notices, err := service.Client.Notification.Query().
			Where(
				notification.ReferenceIDEQ(assignment.ID),
				notification.TitleEQ("发布审核结果"),
			).
			Count(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if audits != events || notices != events {
			t.Fatalf("got %d review audits and %d notifications, want %d each", audits, notices, events)
		}
	}

	if approved {
		requestReview(`{"approved":true,"submissions":[{"platform":"dy","revision":1}]}`, http.StatusConflict)
		assertReview(enum.SubmissionPending, enum.AssignmentPendingReview, "", 0)

		requestReview(`{"approved":true,"submissions":[{"platform":"dy","revision":1},{"platform":"xhs","revision":2}]}`, http.StatusConflict)
		assertReview(enum.SubmissionPending, enum.AssignmentPendingReview, "", 0)

		request := `{"approved":true,"submissions":[{"platform":"dy","revision":1},{"platform":"xhs","revision":1}]}`
		requestReview(request, http.StatusOK)
		requestReview(request, http.StatusOK)
		assertReview(enum.SubmissionApproved, enum.AssignmentPendingSettlement, "", 2)
		return
	}

	requestReview(`{"submissions":[{"platform":"dy","revision":1},{"platform":"xhs","revision":2}],"reason":"补齐必带话题"}`, http.StatusConflict)
	assertReview(enum.SubmissionPending, enum.AssignmentPendingReview, "", 0)

	requestReview(`{"submissions":[{"platform":"dy","revision":1},{"platform":"dy","revision":1}],"reason":"补齐必带话题"}`, http.StatusBadRequest)
	assertReview(enum.SubmissionPending, enum.AssignmentPendingReview, "", 0)

	request := `{"submissions":[{"platform":"dy","revision":1},{"platform":"xhs","revision":1}],"reason":"补齐必带话题"}`
	requestReview(request, http.StatusOK)
	requestReview(request, http.StatusOK)
	assertReview(enum.SubmissionRejected, enum.AssignmentRejected, "补齐必带话题", 2)
}
