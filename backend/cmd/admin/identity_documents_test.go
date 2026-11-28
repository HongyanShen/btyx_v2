package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestIdentityDocumentsRequireSeparateOperatorPermission(t *testing.T) {
	service := testkit.Service(t, "../../..")
	bootstrapTestAdmin(t, service)
	ctx := context.Background()
	root, err := service.LoginAdmin(ctx, testAdminUsername, testAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	member, err := service.DemoLogin(ctx, "identity-documents-preview")
	if err != nil {
		t.Fatal(err)
	}

	createPhoto := func(ownerID, key string) string {
		t.Helper()

		record, err := service.Client.FileObject.Create().
			SetOwnerID(ownerID).
			SetName("identity.jpg").
			SetMediaType(enum.FileJPEG).
			SetSize(100).
			SetStorage(enum.FileStorageTOS).
			SetBucket("fixture").
			SetObjectKey(key).
			SetVersionID("frozen").
			SetVerifiedAt(service.Now()).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return record.ID
	}

	front := createPhoto(member.UserID, "front")
	back := createPhoto(member.UserID, "back")
	encrypt := func(value string) string {
		t.Helper()

		result, err := service.Encrypt(value)
		if err != nil {
			t.Fatal(err)
		}

		return result
	}

	const rawIdentity = "990000199001010017"
	const rawPhone = "13812348888"
	err = service.Client.UserVerification.Create().
		SetUserID(member.UserID).
		SetNameCipher(encrypt("旧档案")).
		SetIdentityCipher(encrypt(rawIdentity)).
		SetVerifiedPhoneCipher(encrypt(rawPhone)).
		SetWechatCipher(encrypt("example_wechat")).
		SetFrontFileID(front).
		SetBackFileID(back).
		SetVerifiedAt(service.Now()).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Client.AuditEvent.Create().
		SetActorID(member.UserID).
		SetReferenceID(member.UserID).
		SetAction(enum.AuditUserIdentityVerify).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	store := &identityPreviewStore{}
	service.AuditService.Store = store
	router := testAdminRouter(t, service)

	request := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()

		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		setAdminSession(req, token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s: got %d, want %d", method, path, response.Code, status)
		}

		return response
	}

	request("POST", "/operators", root.Token,
		`{"username":"identity-reader","password":"test-reader-password","pages":["/admin/realname"],"endpoints":["GET /identity-verifications"]}`, 200)
	reader, err := service.LoginAdmin(ctx, "identity-reader", "test-reader-password")
	if err != nil {
		t.Fatal(err)
	}

	request("GET", "/identity-verifications/documents", reader.Token, "", 403)
	list := request("GET", "/identity-verifications", reader.Token, "", 200)
	if !strings.Contains(list.Body.String(), `"details":null`) || len(store.references) != 0 {
		t.Fatal("base identity history exposed personal details")
	}

	request("POST", "/operators/"+reader.UserID, root.Token,
		`{"pages":["/admin/realname","/users"],"endpoints":["GET /identity-verifications","GET /identity-verifications/documents","GET /users"],"disabled":false}`, 200)

	legacy := request("GET", "/identity-verifications/documents", reader.Token, "", 200)
	var history struct {
		Data audit.IdentityRecordsDTO `json:"data"`
	}
	if err := json.Unmarshal(legacy.Body.Bytes(), &history); err != nil || len(history.Data.Records) != 1 {
		t.Fatal("expected one historical record")
	}

	if details := history.Data.Records[0].Details; details == nil || *details != (audit.IdentityRecordDetailsDTO{}) || len(store.references) != 0 {
		t.Fatal("a historical audit borrowed the account's current identity")
	}

	service.IdentityService.Verifier = snapshotVerifier{}
	err = service.VerifyIdentity(ctx, member.UserID, identity.VerifyIdentityInput{
		DocumentInput: identity.DocumentInput{
			Name:        "张三丰",
			Identity:    rawIdentity,
			FrontFileID: front,
			BackFileID:  back,
		},
		CorrectIdentity: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	result := request("GET", "/identity-verifications/documents", reader.Token, "", 200)
	body := result.Body.String()
	for _, value := range []string{"张**", "9900**********0017", `"phone":""`, `"wechat":""`, "https://images.example.test/front", "https://images.example.test/back"} {
		if !strings.Contains(body, value) {
			t.Fatal("restricted list is missing a masked field or a photo")
		}
	}

	if strings.Contains(body, rawIdentity) || strings.Contains(body, rawPhone) ||
		strings.Contains(body, "138****8888") || strings.Contains(body, "example_wechat") ||
		strings.Contains(body, "张三丰") || result.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("restricted identity list exposed raw data or permitted caching")
	}

	if len(store.references) != 2 || store.references[0].VersionID != "frozen" || store.references[1].VersionID != "frozen" {
		t.Fatal("identity previews did not sign both verified object versions")
	}

	history.Data.Records = nil
	if err := json.Unmarshal(result.Body.Bytes(), &history); err != nil || len(history.Data.Records) != 2 {
		t.Fatal("expected a legacy and a new record for the same user")
	}

	if latest, old := history.Data.Records[0].Details, history.Data.Records[1].Details; latest == nil ||
		latest.Name != "张**" || old == nil || *old != (audit.IdentityRecordDetailsDTO{}) {
		t.Fatal("a new submission populated a previous record")
	}

	err = service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentPending).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	enrollmentDocumentsPath := "/users/" + member.UserID + "/enrollment-documents"
	request("GET", enrollmentDocumentsPath, reader.Token, "", 403)

	const listPath = "/users?view=enrollments&page=1"
	const previewPath = listPath + "&photos=1"
	request("GET", previewPath, reader.Token, "", 403)
	plain := request("GET", listPath, root.Token, "", 200)
	if strings.Contains(plain.Body.String(), "images.example.test") {
		t.Fatal("the standard enrollment list exposed signed photos")
	}

	request("POST", "/operators/"+reader.UserID, root.Token,
		`{"pages":["/admin/realname","/users"],"endpoints":["GET /identity-verifications","GET /identity-verifications/documents","GET /users","GET /users/:id/enrollment-documents"],"disabled":false}`, 200)
	previews := request("GET", previewPath, reader.Token, "", 200)
	if !strings.Contains(previews.Body.String(), `"enrollment_photo_previews":{"front_url":"https://images.example.test/front","back_url":"https://images.example.test/back"`) ||
		previews.Header().Get("X-Total-Count") != "1" || previews.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("the protected page did not return signed thumbnails with its pagination")
	}

	profile := request("GET", enrollmentDocumentsPath, root.Token, "", 200)
	if !strings.Contains(profile.Body.String(), `"phone":"138****8888"`) ||
		!strings.Contains(profile.Body.String(), `"wechat":"example_wechat"`) {
		t.Fatal("enrollment profile is missing the member's current contacts")
	}

	request("GET", "/files/"+front+"/download", root.Token, "", 404)

	foreign := createPhoto("another-owner", "foreign")
	if err := service.Client.UserVerification.Update().
		SetBackFileID(foreign).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	partial := request("GET", previewPath, reader.Token, "", 200)
	if !strings.Contains(partial.Body.String(), `"enrollment_photo_previews":{"front_url":"https://images.example.test/front","back_url":""`) ||
		strings.Contains(partial.Body.String(), "https://images.example.test/foreign") {
		t.Fatal("the enrollment preview did not retain its own side or exposed another user's photo")
	}

	store.references = nil
	result = request("GET", "/identity-verifications/documents", root.Token, "", 200)
	if !strings.Contains(result.Body.String(), "https://images.example.test/back") || len(store.references) != 2 {
		t.Fatal("changing current identity replaced a historical photo")
	}

	request("GET", "/files/"+back+"/download", root.Token, "", 404)
	if err := service.Client.FileObject.UpdateOneID(back).SetIdentityPrivate(true).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	store.references = nil
	result = request("GET", "/identity-verifications/documents", reader.Token, "", 200)
	if !strings.Contains(result.Body.String(), "https://images.example.test/back") || len(store.references) != 2 {
		t.Fatal("retiring a current photo erased its authorized historical preview")
	}

	if err := service.Client.FileObject.UpdateOneID(back).SetOwnerID("another-owner").Exec(ctx); err != nil {
		t.Fatal(err)
	}

	store.references = nil
	result = request("GET", "/identity-verifications/documents", root.Token, "", 200)
	if strings.Contains(result.Body.String(), "https://images.example.test/back") || len(store.references) != 1 {
		t.Fatal("identity preview signed a file owned by another member")
	}

	archived := createPhoto(member.UserID, "archived-front")
	oldApplication, err := service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetIsCurrent(false).
		SetStatus(enum.EnrollmentRejected).
		SetNameCipher(encrypt("原报名")).
		SetFrontFileID(archived).
		SetIdentityCheckedAt(service.Now()).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	withHistory := request("GET", previewPath, reader.Token, "", 200)
	var applications struct {
		Data []identity.UserDTO `json:"data"`
	}
	if err := json.Unmarshal(withHistory.Body.Bytes(), &applications); err != nil || len(applications.Data) != 2 {
		t.Fatal("expected both attempts in the operator list")
	}

	for _, attempt := range applications.Data {
		want := "https://images.example.test/front"
		if attempt.EnrollmentID == oldApplication.ID {
			want = "https://images.example.test/archived-front"
		}

		previews := attempt.EnrollmentPhotoPreviews
		if previews == nil || previews.FrontURL != want || previews.BackURL != "" {
			t.Fatal("attempt thumbnails did not match their own documents")
		}
	}

	archivedDetail := request("GET", enrollmentDocumentsPath+"?enrollment_id="+oldApplication.ID, reader.Token, "", 200)
	if !strings.Contains(archivedDetail.Body.String(), `"name":"原**"`) ||
		!strings.Contains(archivedDetail.Body.String(), `"front_url":"https://images.example.test/archived-front"`) ||
		strings.Contains(archivedDetail.Body.String(), "example_wechat") {
		t.Fatal("archived enrollment detail borrowed current account documents or contacts")
	}

	service.AuditService.Store = nil
	result = request("GET", "/identity-verifications/documents", reader.Token, "", 200)
	if !strings.Contains(result.Body.String(), `"photos_unavailable":true`) ||
		!strings.Contains(result.Body.String(), "9900**********0017") {
		t.Fatal("missing storage must leave the masked details visible and report unavailable photos")
	}
}

type snapshotVerifier struct{}

func (snapshotVerifier) Check(context.Context, string, string) error {
	return nil
}

type identityPreviewStore struct {
	objectstore.Gateway
	references []objectstore.Reference
}

func (s *identityPreviewStore) SignDownload(_ context.Context, reference objectstore.Reference, lifetime time.Duration) (string, error) {
	if lifetime != 2*time.Minute {
		panic("unexpected identity preview lifetime")
	}

	s.references = append(s.references, reference)
	return "https://images.example.test/" + reference.Key, nil
}
