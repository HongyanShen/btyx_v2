package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/gin-gonic/gin"
)

func TestAuditParametersStoredOnlyInDatabase(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := t.Context()
	login, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	request := func(method, path, body, contentType string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		setAdminSession(req, login.Token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s status %d, want %d", path, res.Code, status)
		}

		return res.Body.String()
	}

	form := url.Values{
		"csrf":           {csrfToken(login.Token)},
		"online":         {"on"},
		"price":          {"8.10"},
		"original_price": {"7.50"},
		"daily_limit":    {"03"},
		"creative_limit": {"2"},
	}
	request("POST", "/distribution/settings/marketing", form.Encode(), "application/x-www-form-urlencoded", 303)
	row, err := service.Client.AuditEvent.Query().Where(auditevent.ActionEQ(enum.AuditMarketingSettings)).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if row.Platform != enum.AuditPlatformAdmin {
		t.Fatal("admin form did not capture its HTTP platform")
	}

	if !strings.HasPrefix(row.Parameters, "enc:v1:") {
		t.Fatal("audit form parameters must be encrypted")
	}

	var snapshot struct {
		Query string
		Path  map[string]string
		Body  string
		Form  url.Values `json:"multipart_fields"`
	}
	decode := func(event *ent.AuditEvent) string {
		t.Helper()
		if !strings.HasPrefix(event.Parameters, "enc:v1:") {
			t.Fatal("audit parameters are not encrypted")
		}

		plain := service.Decrypt(strings.TrimPrefix(event.Parameters, "enc:v1:"))
		snapshot.Path = nil
		snapshot.Form = nil
		snapshot.Body = ""
		if err := json.Unmarshal([]byte(plain), &snapshot); err != nil {
			t.Fatal("audit parameters could not be decrypted and decoded")
		}

		return plain
	}
	decode(row)
	if snapshot.Body != form.Encode() {
		t.Fatal("audit did not preserve the complete form body")
	}

	const rawQuery = "tag=b&tag=a&space=a%20b&empty="
	payload := strings.Repeat(" ", 8192) + `{ "name" : "审计标签" }`
	request("POST", "/api/v1/marketing-tags?"+rawQuery, payload, "application/json", 200)
	row, err = service.Client.AuditEvent.Query().Where(auditevent.EndpointEQ("POST /api/v1/marketing-tags")).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	decode(row)
	if snapshot.Query != rawQuery || snapshot.Body != payload {
		t.Fatal("raw query or JSON body changed")
	}

	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	for key, values := range (url.Values{
		"csrf":          {csrfToken(login.Token)},
		"name":          {"新审计标签"},
		"previous_name": {"审计标签"},
		"extra":         {"first", "second"},
	}) {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
	}

	file, err := writer.CreateFormFile("attachment", "audit-file-name.txt")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = file.Write([]byte("audit-file-content")); err != nil {
		t.Fatal(err)
	}

	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	tagID := row.ReferenceID
	request("POST", "/users/tags/"+tagID+"/rename", upload.String(), writer.FormDataContentType(), 303)
	row, err = service.Client.AuditEvent.Query().Where(auditevent.ActionEQ(enum.AuditMarketingTagRename)).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	plain := decode(row)
	if snapshot.Path["id"] != tagID || snapshot.Path["action"] != "rename" ||
		!slices.Equal(snapshot.Form["extra"], []string{"first", "second"}) ||
		strings.Contains(plain, "audit-file") || strings.Contains(plain, "attachment") {
		t.Fatal("multipart audit lost path/fields or retained a file")
	}

	html := request("GET", "/audits", "", "", 200)
	if !strings.Contains(html, `name="actor_name"`) || !strings.Contains(html, `name="platform"`) ||
		!strings.Contains(html, `name="endpoint"`) || !strings.Contains(html, "<td>管理后台</td>") {
		t.Fatal("missing audit filters or recorded admin platform")
	}

	_, table, ok := strings.Cut(html, `aria-label="操作审计表格"`)
	if !ok || strings.Contains(table, "操作接口") || strings.Contains(html, "操作参数") || strings.Contains(table, "POST /api/v1/marketing-tags") {
		t.Fatal("audit table exposed technical fields")
	}

	api := request("GET", "/api/v1/audits", "", "", 200)
	if strings.Contains(api, `"parameters":`) || strings.Contains(api, `"endpoint":`) {
		t.Fatal("audit JSON exposed technical fields")
	}

	filters := url.Values{
		"actor_name": {strings.ToUpper(testAdminUsername[:3])},
		"platform":   {"admin"},
		"endpoint":   {"POST /api/v1/marketing-tags"},
	}
	filtered := request("GET", "/api/v1/audits?"+filters.Encode(), "", "", 200)
	if !strings.Contains(filtered, tagID) || !strings.Contains(filtered, `"platform":"admin"`) || strings.Contains(filtered, string(enum.AuditMarketingTagRename)) {
		t.Fatal("combined audit filters did not select the admin API operation")
	}

	for key, value := range map[string]string{
		"actor_name": "没有这个操作人",
		"platform":   "miniapp",
		"endpoint":   "POST /missing",
	} {
		other := url.Values{key: {value}}
		empty := request("GET", "/audits?"+other.Encode(), "", "", 200)
		if !strings.Contains(empty, "没有符合条件的审计记录") {
			t.Fatalf("audit filter %s did not exclude unrelated rows", key)
		}
	}

	request("GET", "/api/v1/audits?platform=other", "", "", 400)

	failedBody := request("POST", "/api/v1/marketing-tags", `{"name":""}`, "application/json", 400)
	result := request("GET", "/api/v1/audits?"+filters.Encode(), "", "", 200)
	if !strings.Contains(result, `"http_status":200`) || !strings.Contains(result, `"http_status":400`) {
		t.Fatal("audit results must reflect the HTTP status returned to the caller")
	}

	var cipher string
	err = service.DB.QueryRowContext(ctx, `SELECT r.body_cipher FROM audit_http_responses r
		JOIN audit_events e ON e.request_id = r.id WHERE e.action = 'request.failed' AND r.status = 400`).Scan(&cipher)
	if err != nil || !strings.HasPrefix(cipher, "enc:v1:") || service.Decrypt(strings.TrimPrefix(cipher, "enc:v1:")) != failedBody {
		t.Fatalf("encrypted response did not match the actual JSON response: %v", err)
	}

	resultsPage := request("GET", "/audits?"+filters.Encode(), "", "", 200)
	if !strings.Contains(resultsPage, "接口结果") || !strings.Contains(resultsPage, ">失败<") || !strings.Contains(resultsPage, ">成功<") {
		t.Fatal("audit table did not display both operation outcomes")
	}
}

func TestAuditHTTPResultIndependentOfBusinessOutcomeAndFileBody(t *testing.T) {
	service := testkit.Service(t, "../../..")
	router := testAdminRouter(t, service)
	const body = "response as sent"
	router.POST("/audit-test/:format", func(ctx *gin.Context) {
		// The same business event can be followed by different HTTP responses.
		if err := service.Client.AuditEvent.Create().
			SetActorID("admin").
			SetReferenceID("test").
			SetAction(enum.AuditUserIdentityFailed).
			Exec(ctx.Request.Context()); err != nil {
			t.Fatal(err)
		}

		if ctx.Param("format") == "file" {
			ctx.Header("Content-Disposition", "attachment; filename=report.txt")
			ctx.Data(200, "text/plain", []byte(body))
			return
		}

		ctx.Header("Content-Type", "text/html; charset=utf-8")
		if _, err := ctx.Writer.WriteString("response "); err != nil {
			t.Fatal(err)
		}

		if _, err := ctx.Writer.Write([]byte("as sent")); err != nil {
			t.Fatal(err)
		}
	})
	for _, format := range []string{"html", "file"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("POST", "/audit-test/"+format, nil))
		stored, err := service.Client.AuditHTTPResponse.Get(t.Context(), res.Header().Get("X-Request-ID"))
		if err != nil || stored.Status != 200 || res.Body.String() != body || stored.IsFile != (format == "file") {
			t.Fatalf("unexpected %s response metadata: %v", format, err)
		}

		if stored.IsFile && stored.BodyCipher != "" {
			t.Fatal("file response body was retained")
		}

		if !stored.IsFile && service.Decrypt(strings.TrimPrefix(stored.BodyCipher, "enc:v1:")) != body {
			t.Fatal("HTML response bytes did not round-trip")
		}
	}

	rows, err := service.Audits(t.Context())
	if err != nil || len(rows) != 2 || rows[0].HTTPStatus != 200 || rows[1].HTTPStatus != 200 {
		t.Fatal("audit list inferred a result from the business action")
	}
}

func TestAuditQueryBeyondRecentRecords(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := t.Context()
	login, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	stamp := time.Date(2026, 11, 23, 18, 30, 0, 0, beijing)
	var batch []*ent.AuditEventCreate
	for n := range 305 {
		batch = append(batch, service.Client.AuditEvent.Create().
			SetAction(enum.AuditTaskVisibility).
			SetActorID(login.UserID).
			SetEndpoint("POST /tasks/:id/:action").
			SetReferenceID(fmt.Sprintf("audit-row-%03d", n)).
			SetCreatedAt(stamp.Add(time.Duration(n)*time.Minute)))
	}

	if err := service.Client.AuditEvent.CreateBulk(batch...).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	router := testAdminRouter(t, service)
	query := func(path string, status int) string {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		setAdminSession(req, login.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("audit query status %d, want %d", response.Code, status)
		}

		return response.Body.String()
	}

	oldest := query("/audits?page=21", 200)
	if !strings.Contains(oldest, "audit-row-000") {
		t.Fatal("oldest audit missing after the first 300 records")
	}

	filtered := query("/audits?page=2&start=2026-11-23T18:30&end=2026-11-23T23:59", 200)
	if !strings.Contains(filtered, "共 305 条") || !strings.Contains(filtered, "audit-row-289") ||
		!strings.Contains(filtered, "page=3&amp;end=2026-11-23T23%3A59&amp;start=2026-11-23T18%3A30") {
		t.Fatal("filtered page or pagination links lost the query")
	}

	boundary := query("/audits?start=2026-11-23T18:30&end=2026-11-23T18:30", 200)
	if !strings.Contains(boundary, "audit-row-000") || strings.Contains(boundary, "audit-row-001") {
		t.Fatal("audit query did not respect the Beijing minute range")
	}

	query("/audits?start=2026-11-24T12:00&end=2026-11-23T12:00", 400)

	combined := url.Values{
		"actor_name": {"TEST-"},
		"platform":   {"admin"},
		"endpoint":   {"POST /tasks/:id/:action"},
	}
	paged := query("/audits?page=21&"+combined.Encode(), 200)
	if !strings.Contains(paged, "audit-row-000") || !strings.Contains(paged, "<td>管理后台</td>") ||
		!strings.Contains(html.UnescapeString(paged), "page=20&"+combined.Encode()) {
		t.Fatal("historical platform filtering or pagination did not preserve the combined filters")
	}

	member, err := service.Client.User.Create().SetNickname("小白兔运营").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Client.AuditEvent.Create().
		SetActorID(member.ID).
		SetAction(enum.AuditUserProfileUpdate).
		SetReferenceID("legacy-member-event").
		SetEndpoint("PUT /api/v1/me").
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	memberPage := query("/audits?platform=miniapp&actor_name="+url.QueryEscape("白兔"), 200)
	if !strings.Contains(memberPage, "<td>小程序</td>") || !strings.Contains(memberPage, "legacy-member-event") || !strings.Contains(memberPage, "共 1 条") {
		t.Fatal("historical miniapp operation missing from nickname and platform search")
	}
}

func TestAuditMigrationKeepsHistoricalEvents(t *testing.T) {
	service := testkit.Service(t, "../../..")
	ctx := t.Context()
	old, err := service.Client.AuditEvent.Create().
		SetAction(enum.AuditTaskVisibility).
		SetActorID("old-operator").
		SetReferenceID("old-task").
		SetReason("历史备注").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.DB.ExecContext(ctx, "ALTER TABLE audit_events DROP COLUMN endpoint, DROP COLUMN parameters, DROP COLUMN platform"); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DB.ExecContext(ctx, `ALTER TABLE audit_events DROP CONSTRAINT audit_action_enum_v7,
		ADD CONSTRAINT audit_action_enum_v6 CHECK (action <> 'request.failed')`); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	row, err := service.Client.AuditEvent.Get(ctx, old.ID)
	if err != nil || row.Reason != old.Reason || row.Endpoint != "" || row.Parameters != "" || row.Platform != "" {
		t.Fatalf("historical audit changed: %v", err)
	}

	if err := service.Client.AuditEvent.Create().
		SetActorID("admin").
		SetReferenceID(old.ReferenceID).
		SetAction(enum.AuditRequestFailed).
		Exec(ctx); err != nil {
		t.Fatalf("failed audit rejected after upgrading the old constraint: %v", err)
	}
}
