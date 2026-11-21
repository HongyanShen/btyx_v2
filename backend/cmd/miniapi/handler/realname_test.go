package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

const adultIdentity = "990000199001010017"

type verifyDocumentFunc func(context.Context, string, string) error

func (verify verifyDocumentFunc) Check(ctx context.Context, name, number string) error {
	return verify(ctx, name, number)
}

func addIdentityPhoto(t *testing.T, services *business.Service, ownerID string) string {
	t.Helper()
	record, err := services.Client.FileObject.Create().
		SetOwnerID(ownerID).
		SetName("watermarked.jpg").
		SetObjectKey("images/" + ownerID).
		SetBucket("test").
		SetSize(1024).
		SetStorage(enum.FileStorageTOS).
		SetMediaType(enum.FileJPEG).
		SetVerifiedAt(services.Now()).
		Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	return record.ID
}

func TestMiniProfileReturnsCurrentIdentityOrResubmissionHint(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	services.IdentityService.Verifier = verifyDocumentFunc(func(_ context.Context, _, number string) error {
		if number != adultIdentity {
			t.Error("verification did not get the submitted or retained number")
		}

		return nil
	})
	router := minihandler.NewRouter(services)
	login, err := services.DemoLogin(ctx, "identity-readback")
	if err != nil {
		t.Fatal(err)
	}

	other, err := services.DemoLogin(ctx, "identity-readback-unrelated")
	if err != nil {
		t.Fatal(err)
	}

	front := addIdentityPhoto(t, services, login.UserID)
	back := addIdentityPhoto(t, services, login.UserID)
	post := func(path, body string, status int) {
		t.Helper()

		request := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+login.Token)
		request.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, request)
		if result.Code != status {
			t.Fatalf("POST %s returned %d, want %d", path, result.Code, status)
		}
	}

	post("/me/identity",
		`{"name":"测试实名","identity":"`+adultIdentity+`","front_file_id":"`+front+`","back_file_id":"`+back+`","contact_phone":"13700001111","wechat":"submitted-wechat"}`,
		http.StatusOK)

	type identityProfile struct {
		Verified             bool   `json:"identity_verified"`
		ResubmissionRequired bool   `json:"identity_resubmission_required"`
		Name                 string `json:"name"`
		ContactPhone         string `json:"contact_phone"`
		VerifiedPhone        string `json:"verified_phone"`
		Document             *struct {
			Number      string `json:"number"`
			FrontFileID string `json:"front_file_id"`
			BackFileID  string `json:"back_file_id"`
		} `json:"identity_document"`
	}
	profile := func(token string) identityProfile {
		t.Helper()

		request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("identity profile did not return a private response")
		}

		var body struct {
			Data identityProfile `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}

		if strings.Contains(response.Body.String(), adultIdentity) {
			t.Fatal("identity profile exposed the full document number")
		}

		if strings.Contains(response.Body.String(), "13700001111") {
			t.Fatal("identity profile exposed the full manual phone number")
		}

		return body.Data
	}

	saved := profile(login.Token)
	if !saved.Verified || saved.ResubmissionRequired || saved.Name != "测试实名" || saved.Document == nil ||
		saved.Document.Number != "9900**********0017" || saved.Document.FrontFileID != front || saved.Document.BackFileID != back ||
		saved.ContactPhone != "137****1111" || saved.VerifiedPhone != "" {
		t.Fatal("self profile did not return the current saved identity")
	}

	storedContact, err := services.Client.UserVerification.Query().
		Where(userverification.UserIDEQ(login.UserID)).
		Only(ctx)
	if err != nil || storedContact.ContactPhoneCipher == "13700001111" ||
		services.Decrypt(storedContact.ContactPhoneCipher) != "13700001111" {
		t.Fatal("manual contact phone was not stored as ciphertext")
	}

	unrelated := profile(other.Token)
	if unrelated.ResubmissionRequired || unrelated.Document != nil {
		t.Fatal("unverified user received identity history or photos from another account")
	}

	nextBack := addIdentityPhoto(t, services, login.UserID)
	post("/me/identity",
		`{"correct_identity":true,"name":"更正后的实名","front_file_id":"`+front+`","back_file_id":"`+nextBack+`"}`,
		http.StatusOK)
	corrected := profile(login.Token)
	if corrected.Document == nil || corrected.Document.FrontFileID != front || corrected.Document.BackFileID != nextBack ||
		corrected.Name != "更正后的实名" || corrected.ContactPhone != saved.ContactPhone {
		t.Fatal("correction did not retain the number, front image, and contact details")
	}

	services.AuditService.Store = nil
	history, err := services.IdentityRecordsWithDocuments(ctx, "")
	if err != nil || len(history.Records) != 2 {
		t.Fatal("expected the original and corrected verification records")
	}

	if details := history.Records[0].Details; details == nil || details.Phone != "" || details.Wechat != "" {
		t.Fatal("correction borrowed contact details from another submission")
	}

	if details := history.Records[1].Details; details == nil || details.Phone != "137****1111" || details.Wechat != "submitted-wechat" {
		t.Fatal("original verification did not retain its explicitly submitted contacts")
	}

	if old, updated := history.Records[1].Details, history.Records[0].Details; old.Name != "测***" ||
		updated.Name != "更*****" || old.Identity != saved.Document.Number || updated.Identity != saved.Document.Number {
		t.Fatal("history did not preserve the documents used by each verification")
	}

	retained, err := services.Client.FileObject.Get(ctx, front)
	if err != nil || retained.IdentityPrivate {
		t.Fatal("the retained side was made inaccessible")
	}

	replaced, err := services.Client.FileObject.Get(ctx, back)
	if err != nil || !replaced.IdentityPrivate {
		t.Fatal("the replaced side was not retired")
	}

	post("/me/identity",
		`{"correct_identity":true,"name":"更正后的实名","front_file_id":"`+back+`","back_file_id":"`+nextBack+`"}`,
		http.StatusBadRequest)
	post("/me/identity", `{"contact_phone":"123"}`, http.StatusBadRequest)
	post("/me/identity", `{"contact_phone":"13600002222"}`, http.StatusOK)
	manual := profile(login.Token)
	if manual.ContactPhone != "136****2222" || manual.VerifiedPhone != "" || !manual.Verified {
		t.Fatal("manual contact update changed verified phone or identity status")
	}

	services.WechatPhone = verifyPhoneFunc(func(context.Context, string) (string, error) {
		return "13800003333", nil
	})
	post("/me/phone", `{"code":"fixture-code"}`, http.StatusOK)
	if profile(login.Token).ContactPhone != "136****2222" {
		t.Fatal("profile's independent WeChat authorization must leave manual contact intact")
	}

	post("/me/phone", `{"code":"fixture-code","use_for_contact":true}`, http.StatusOK)
	adopted := profile(login.Token)
	if adopted.ContactPhone != "138****3333" || adopted.VerifiedPhone != "138****3333" || !adopted.Verified {
		t.Fatal("WeChat authorization did not replace the manual contact preference")
	}

	authorization, err := services.VerifyPhone(ctx, login.UserID, "fixture-code", true)
	if err != nil || authorization.ContactPhoneReceipt == "" {
		t.Fatal("WeChat authorization did not issue a receipt for the selected number")
	}

	post("/me/identity", `{"contact_phone_receipt":"invalid"}`, http.StatusBadRequest)
	authorizedBack := addIdentityPhoto(t, services, login.UserID)
	post("/me/identity",
		`{"correct_identity":true,"name":"再次更正","front_file_id":"`+front+`","back_file_id":"`+authorizedBack+`","contact_phone_receipt":"`+authorization.ContactPhoneReceipt+`"}`,
		http.StatusOK)
	history, err = services.IdentityRecordsWithDocuments(ctx, "")
	if err != nil || len(history.Records) == 0 ||
		history.Records[0].Details == nil || history.Records[0].Details.Phone != "138****3333" ||
		history.Records[0].Details.Wechat != "" {
		t.Fatal("verification did not record exactly the number authorized for that submission")
	}

	if err := services.Client.UserVerification.Update().
		Where(userverification.UserIDEQ(login.UserID)).
		SetNameCipher("").
		SetIdentityCipher("").
		SetFrontFileID("").
		SetBackFileID("").
		ClearVerifiedAt().
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	missing := profile(login.Token)
	if missing.Verified || !missing.ResubmissionRequired || missing.Document != nil {
		t.Fatal("a historical check should request resubmission without certifying the user")
	}
}

func TestRealNameAndEnrollmentAreIndependent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	services := testkit.Service(t, "../../../..")
	terms, err := services.Client.Agreement.Create().
		SetTitle("营销分发任务协议").
		SetBody("报名须确认本协议").
		SetCreatorID("admin").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	router := minihandler.NewRouter(services)
	var verificationError error
	var duringCheck func()
	var checks int
	expectedName := "独立实名"
	verifier := verifyDocumentFunc(func(_ context.Context, name, number string) error {
		checks++
		if name != expectedName || number != adultIdentity {
			t.Error("verification did not receive the expected pair")
		}

		if duringCheck != nil {
			action := duringCheck
			duringCheck = nil
			action()
		}

		return verificationError
	})
	services.IdentityService.Verifier = verifier
	services.EnrollmentService.Verifier = verifier

	login, err := services.DemoLogin(ctx, "separate-verification")
	if err != nil {
		t.Fatal(err)
	}

	post := func(path, token, body string, want int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("POST %s returned status %d, want %d", path, response.Code, want)
		}
	}

	front := addIdentityPhoto(t, services, login.UserID)
	back := addIdentityPhoto(t, services, login.UserID)
	services.IdentityOCR = nil
	recognition := `{"file_id":"` + front + `"}`
	post("/me/identity/recognize", login.Token, recognition, http.StatusServiceUnavailable)

	identity := `"name":"独立实名", "identity":"` + adultIdentity + `", "front_file_id":"` + front + `","back_file_id":"` + back + `"`
	agreementField := `,"agreement_id":"` + terms.ID + `"`
	platforms := `"douyin":"test-dy","xhs":"xhs_2026"` + agreementField
	legacyLink := `"douyin":"test-dy","xhs":"https://www.xiaohongshu.com/user/profile/test"` + agreementField
	post("/enrollment", login.Token, "{"+legacyLink+","+identity+"}", http.StatusBadRequest)
	post("/enrollment", login.Token, "{"+platforms+"}", http.StatusBadRequest)
	verificationError = app.Invalid("姓名和身份证号不一致")
	post("/enrollment", login.Token, "{"+platforms+","+identity+"}", http.StatusBadRequest)
	verificationError = nil
	post("/enrollment", login.Token, "{"+platforms+","+identity+`,"tags":["时尚"]}`, http.StatusBadRequest)
	post("/enrollment", login.Token, "{"+platforms+","+identity+`,"phone":"13600001111"}`, http.StatusOK)
	post("/enrollment", login.Token, "{"+platforms+","+identity+`,"phone":"13600001111"}`, http.StatusOK)

	services.AuditService.Store = nil
	submissions, err := services.IdentityRecordsWithDocuments(ctx, "")
	if err != nil || len(submissions.Records) != 3 || submissions.Records[0].Details == nil ||
		submissions.Records[0].Details.Phone != "136****1111" || submissions.Records[0].Details.Wechat != "" {
		t.Fatal("enrollment verification did not record only the phone actually submitted")
	}

	failed := submissions.Records[1]
	empty := submissions.Records[2]
	if failed.Action != enum.AuditEnrollmentIdentityFailed || failed.Details == nil ||
		failed.Details.Name != "独***" || failed.Details.Identity != "9900**********0017" ||
		empty.Details == nil || empty.Details.Name != "" || empty.Details.Identity != "" {
		t.Fatal("enrollment failures did not preserve the available details from each submission")
	}

	profile, err := services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentPending || profile.IdentityVerified {
		t.Fatalf("pending enrollment certified identity: err=%v", err)
	}

	_, fileReadErr := services.FileForRead(ctx, "staff", enum.RoleAdmin, front)
	var denied *app.Error
	if !errors.As(fileReadErr, &denied) || denied.Status != http.StatusNotFound {
		t.Fatal("admin can download an identity document")
	}

	if _, err := services.FileForRead(ctx, login.UserID, enum.RoleUser, front); err != nil {
		t.Fatal("member cannot access their own identity document")
	}

	pendingVersion := func() time.Time {
		t.Helper()
		record, err := services.Client.UserEnrollment.Query().
			Where(userenrollment.UserIDEQ(login.UserID), userenrollment.IsCurrentEQ(true)).
			Only(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return record.UpdatedAt
	}

	originalVersion := pendingVersion()
	if err := services.ReviewEnrollment(ctx, "staff", login.UserID, false, "请重新上传", nil, &originalVersion); err != nil {
		t.Fatal(err)
	}

	profile, err = services.User(ctx, login.UserID)
	if err != nil || profile.IdentityVerified || profile.Status != enum.EnrollmentRejected ||
		profile.EnrollmentDocument == nil || profile.EnrollmentDocument.FrontFileID != front ||
		profile.EnrollmentDocument.BackFileID != back {
		t.Fatalf("rejected enrollment did not expose its own reusable photos: err=%v", err)
	}

	rejectedID := profile.EnrollmentID
	retainedPhotos := `"front_file_id":"` + front + `","back_file_id":"` + back + `"`
	post("/enrollment", login.Token, "{"+platforms+","+identity+`,"phone":"13600001111"}`, http.StatusConflict)
	post("/enrollment", login.Token, "{"+platforms+","+retainedPhotos+`,"previous_enrollment_id":"`+rejectedID+`"}`, http.StatusOK)
	retained, err := services.Client.UserEnrollment.Get(ctx, rejectedID)
	if err != nil || retained.Status != enum.EnrollmentRejected || retained.Reason != "请重新上传" ||
		retained.FrontFileID != front || retained.BackFileID != back || retained.IsCurrent ||
		services.Decrypt(retained.ContactPhoneCipher) != "13600001111" {
		t.Fatalf("original rejection or its documents changed: err=%v", err)
	}

	if err := services.ReviewEnrollment(ctx, "staff", login.UserID, true, "", nil, &originalVersion); err == nil {
		t.Fatal("stale review approved a newer application")
	}

	// Model a pending application from before automatic checks on submission.
	if err := services.Client.UserEnrollment.Update().
		Where(userenrollment.UserIDEQ(login.UserID), userenrollment.IsCurrentEQ(true)).
		ClearIdentityCheckedAt().
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	currentVersion := pendingVersion()
	verificationError = app.Invalid("姓名和身份证号不一致")
	if err := services.ReviewEnrollment(ctx, "staff", login.UserID, true, "", nil, &currentVersion); !errors.Is(err, verificationError) {
		t.Fatal("approval must require a positive identity check")
	}

	profile, err = services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentPending || profile.IdentityVerified {
		t.Fatal("a failed identity check changed the pending application")
	}

	verificationError = nil
	duringCheck = func() {
		if err := services.ReviewEnrollment(ctx, "staff", login.UserID, false, "资料有更新", nil, &currentVersion); err != nil {
			t.Fatal(err)
		}

		rejected, err := services.User(ctx, login.UserID)
		if err != nil || rejected.Status != enum.EnrollmentRejected {
			t.Fatal("expected second attempt to be rejected")
		}

		post("/enrollment", login.Token, "{"+platforms+","+identity+`,"previous_enrollment_id":"`+rejected.EnrollmentID+`"}`, http.StatusOK)
	}
	err = services.ReviewEnrollment(ctx, "staff", login.UserID, true, "", nil, &currentVersion)
	var stale *app.Error
	if !errors.As(err, &stale) || stale.Status != http.StatusConflict {
		t.Fatal("approval must reject updates made while waiting for the verifier")
	}

	currentVersion = pendingVersion()
	if err := services.ReviewEnrollment(ctx, "staff", login.UserID, true, "", nil, &currentVersion); err != nil {
		t.Fatal(err)
	}

	if checks != 6 {
		t.Fatal("approval must check only the current pending application")
	}

	profile, err = services.User(ctx, login.UserID)
	if err != nil || profile.Status != enum.EnrollmentApproved || !profile.IdentityVerified {
		t.Fatalf("approved enrollment did not certify identity: err=%v", err)
	}

	other, err := services.DemoLogin(ctx, "realname-only")
	if err != nil {
		t.Fatal(err)
	}

	post("/me/identity/recognize", other.Token, recognition, http.StatusNotFound)

	otherFront := addIdentityPhoto(t, services, other.UserID)
	otherBack := addIdentityPhoto(t, services, other.UserID)
	ownPhotos := strings.NewReplacer(front, otherFront, back, otherBack).Replace(identity)
	post("/me/identity", other.Token, "{"+identity+"}", http.StatusBadRequest)
	underage := strings.Replace(ownPhotos, adultIdentity, "990000202001010011", 1)
	post("/me/identity", other.Token, "{"+underage+"}", http.StatusBadRequest)
	expectedName = "更正姓名"
	corrected := strings.Replace(ownPhotos, "独立实名", expectedName, 1)
	verificationError = &app.Error{
		Status: http.StatusServiceUnavailable,
		Code:   enum.ErrorUnavailable,
	}
	post("/me/identity", other.Token, "{"+corrected+"}", http.StatusServiceUnavailable)

	profile, err = services.User(ctx, other.UserID)
	if err != nil || profile.IdentityVerified {
		t.Fatal("service failure certified the member")
	}

	verificationError = nil
	post("/me/identity", other.Token, "{"+corrected+"}", http.StatusOK)
	post("/me/identity", other.Token, "{"+corrected+"}", http.StatusOK)
	if checks != 8 {
		t.Fatal("invalid input or repeat submission invoked the provider")
	}

	profile, err = services.User(ctx, other.UserID)
	if err != nil || profile.Status != enum.EnrollmentNone || !profile.IdentityVerified {
		t.Fatalf("standalone verification affected enrollment: err=%v", err)
	}

	post("/enrollment", other.Token, "{"+platforms+"}", http.StatusOK)
	if err := services.ReviewEnrollment(ctx, "staff", other.UserID, false, "账号资料需更新", nil, nil); err != nil {
		t.Fatal(err)
	}

	profile, err = services.User(ctx, other.UserID)
	if err != nil || profile.Status != enum.EnrollmentRejected || !profile.IdentityVerified {
		t.Fatalf("enrollment rejection affected standalone verification: err=%v", err)
	}

	history, err := services.IdentityRecords(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	actions := make(map[enum.AuditAction]int)
	for _, record := range history.Records {
		actions[record.Action]++
		if strings.Contains(record.Reason, adultIdentity) {
			t.Fatal("verification history exposed the identity number")
		}
	}

	if actions[enum.AuditUserIdentityVerify] != 1 ||
		actions[enum.AuditUserIdentityFailed] < 1 ||
		actions[enum.AuditEnrollmentIdentityVerify] != 3 ||
		actions[enum.AuditEnrollmentIdentityFailed] < 1 {
		t.Fatal("verification history did not capture both outcomes")
	}

	replacementFront := addIdentityPhoto(t, services, other.UserID)
	replacementBack := addIdentityPhoto(t, services, other.UserID)
	expectedName = "更新后的姓名"
	replacement := strings.NewReplacer(
		"独立实名", expectedName,
		otherFront, replacementFront,
		otherBack, replacementBack,
	).Replace(ownPhotos)
	correction := `{"correct_identity":true,` + replacement + "}"

	verificationError = app.Invalid("姓名和身份证号不一致")
	post("/me/identity", other.Token, correction, http.StatusBadRequest)
	profile, err = services.User(ctx, other.UserID)
	if err != nil || profile.Name != "更正姓名" || !profile.IdentityVerified {
		t.Fatal("failed correction changed the effective identity")
	}

	verificationError = nil
	post("/me/identity", other.Token, correction, http.StatusOK)
	post("/me/identity", other.Token, correction, http.StatusOK)
	profile, err = services.User(ctx, other.UserID)
	if err != nil || profile.Name != expectedName || !profile.IdentityVerified || profile.Status != enum.EnrollmentRejected {
		t.Fatal("successful correction did not replace only the effective identity")
	}

	if checks != 10 {
		t.Fatal("identity correction must be checked and an exact retry must reuse the result")
	}

	for _, fileID := range []string{otherFront, otherBack, replacementFront, replacementBack} {
		_, err := services.FileForRead(ctx, "staff", enum.RoleAdmin, fileID)
		if !errors.As(err, &denied) || denied.Status != http.StatusNotFound {
			t.Fatal("admin can download a current or replaced identity photo")
		}
	}
}
