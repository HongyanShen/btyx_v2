package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestIdentityRecordsRenderHTMLAndPageBothWays(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	operator, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.DemoLogin(ctx, "htmx-identity-records")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 11, 18, 2, 0, 0, 0, time.UTC)
	var firstPageCursor, secondPageStart, secondPageCursor, thirdPageStart string
	for index := range 101 {
		row, err := service.Client.AuditEvent.Create().
			SetID(fmt.Sprintf("%027d", index)).
			SetAction(enum.AuditEnrollmentIdentityFailed).
			SetCreatedAt(base.Add(time.Duration(index/2) * time.Minute)).
			SetActorID(member.UserID).
			SetReferenceID(member.UserID).
			SetReason(fmt.Sprintf("核验记录 %03d <试验>", index)).
			SetSubmittedMaskedName("陈**").
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		switch index {
		case 0:
			thirdPageStart = row.ID
		case 1:
			secondPageCursor = row.ID
		case 50:
			secondPageStart = row.ID
		case 51:
			firstPageCursor = row.ID
		}
	}

	const password = "identity-html-reader-password"
	_, err = service.CreateOperator(ctx, operator.UserID, identity.CreateOperatorInput{
		Username: "identity-html-reader",
		Password: password,
		OperatorGrants: identity.OperatorGrants{
			Pages:     []string{"/admin/realname"},
			Endpoints: []string{"GET /identity-verifications"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	reader, err := service.LoginAdmin(ctx, "identity-html-reader", password)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	request := func(token, query string, partial bool, status int) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin/realname"+query, nil)
		setAdminSession(req, token)
		if partial {
			req.Header.Set("HX-Request", "true")
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("identity page: status %d, cache control %q", response.Code, response.Header().Get("Cache-Control"))
		}

		return response.Body.String()
	}

	first := request(reader.Token, "", false, http.StatusOK)
	if !strings.Contains(first, `aria-current="page">1</span>`) ||
		!strings.Contains(first, `aria-label="第 2 页">2</a>`) {
		t.Fatal("identity records must show the current and next page numbers")
	}

	if !strings.Contains(first, "<!doctype html>") || !strings.Contains(first, "报名审核 · 实名认证记录") ||
		!strings.Contains(first, "核验记录 100 &lt;试验&gt;") || strings.Contains(first, "陈**") ||
		strings.Contains(first, "核验记录 000") || strings.Contains(first, "data-workspace-page") ||
		!strings.Contains(first, `hx-get="/admin/realname?page=2"`) {
		t.Fatalf("the first page must render escaped records in HTML, hide details and link to the next cursor")
	}

	second := request(operator.Token, "?before="+firstPageCursor, true, http.StatusOK)
	if strings.Contains(second, "<!doctype html>") || strings.Contains(second, "核验记录 100") ||
		!strings.Contains(second, "陈**") || !strings.Contains(second, "核验记录 050") ||
		!strings.Contains(second, `hx-get="/admin/realname?page=1"`) ||
		!strings.Contains(second, `hx-get="/admin/realname?page=3"`) {
		t.Fatal("the second page must render its own records, permitted details and both cursors")
	}

	third := request(operator.Token, "?before="+secondPageCursor, true, http.StatusOK)
	if !strings.Contains(third, "核验记录 000") || !strings.Contains(third, `aria-label="下一页" disabled`) ||
		!strings.Contains(third, `aria-current="page">3</span>`) {
		t.Fatal("the last page must only link backward")
	}

	back := request(operator.Token, "?after="+thirdPageStart, true, http.StatusOK)
	if !strings.Contains(back, "核验记录 050") || strings.Contains(back, "核验记录 000") ||
		!strings.Contains(back, `hx-get="/admin/realname?page=3"`) ||
		!strings.Contains(back, `hx-get="/admin/realname?page=1"`) {
		t.Fatal("backward pagination must recover the same middle page")
	}

	back = request(operator.Token, "?after="+secondPageStart, true, http.StatusOK)
	if !strings.Contains(back, "核验记录 100") || !strings.Contains(back, `aria-label="上一页" disabled`) {
		t.Fatal("backward pagination must return to the first page")
	}

	invalid := request(operator.Token, "?before=unknown", true, http.StatusBadRequest)
	if !strings.Contains(invalid, "认证记录分页位置无效") {
		t.Fatal("an invalid cursor must show a recoverable error")
	}
}
