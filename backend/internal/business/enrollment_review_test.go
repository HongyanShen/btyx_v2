package business

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/notification"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
)

func TestEnrollmentReviewPreservesRejectedAttempt(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	member, err := service.DemoLogin(ctx, "enrollment-portrait")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareTestUser(ctx, member.UserID); err != nil {
		t.Fatal(err)
	}

	if err := service.Client.UserVerification.Create().
		SetUserID(member.UserID).
		SetVerifiedAt(service.Now()).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	record, err := service.Client.UserEnrollment.Query().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	record, err = service.Client.UserEnrollment.UpdateOne(record).
		SetStatus(enum.EnrollmentPending).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	recentPost, newAccount := false, true
	input := enrollment.ReviewInput{
		Reason:     "主页没有近期作品",
		Tags:       []string{"摄影"},
		UpdatedAt:  &record.UpdatedAt,
		RecentPost: &recentPost,
		NewAccount: &newAccount,
		AgeGroup:   enum.EnrollmentAge25To30,
	}
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err != nil {
		t.Fatal(err)
	}

	rows, err := service.EnrollmentUsers(ctx, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("enrollment list: %v, %v", rows, err)
	}

	rejected := rows[0]
	if rejected.Status != enum.EnrollmentRejected || rejected.Nickname != "" || rejected.EnrollmentRecentPost == nil ||
		*rejected.EnrollmentRecentPost || rejected.EnrollmentNewAccount == nil ||
		!*rejected.EnrollmentNewAccount || rejected.EnrollmentAgeGroup == nil ||
		*rejected.EnrollmentAgeGroup != enum.EnrollmentAge25To30 ||
		len(rejected.Tags) != 1 || rejected.Tags[0] != "摄影" {
		t.Fatalf("portrait not recorded: %+v", rejected)
	}

	input.Approved = true
	input.Reason = ""
	input.Tags = []string{"时尚"}
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err == nil {
		t.Fatal("stale rejection revision was allowed to approve")
	}

	input.UpdatedAt = rejected.EnrollmentUpdatedAt
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err == nil {
		t.Fatal("a rejected decision could be reversed")
	}

	terms, err := service.CurrentAgreement(ctx)
	if err != nil {
		t.Fatal(err)
	}

	application := enrollment.ApplyInput{
		Douyin:      "new-douyin",
		XHS:         "new-xhs",
		AgreementID: terms.ID,
	}
	if err := service.Apply(ctx, member.UserID, application); err == nil {
		t.Fatal("an unconfirmed retry could create a new attempt after rejection")
	}

	application.PreviousEnrollmentID = rejected.EnrollmentID
	if err := service.Apply(ctx, member.UserID, application); err != nil {
		t.Fatal(err)
	}

	if err := service.Apply(ctx, member.UserID, application); err != nil {
		t.Fatalf("identical application retry: %v", err)
	}

	rows, err = service.EnrollmentUsers(ctx, "")
	if err != nil || len(rows) != 2 || rows[0].Status != enum.EnrollmentPending || len(rows[0].Tags) != 0 ||
		rows[0].Douyin != application.Douyin || rows[1].Status != enum.EnrollmentRejected ||
		rows[1].Reason != rejected.Reason || rows[1].Douyin != rejected.Douyin ||
		len(rows[1].Tags) != 1 || rows[1].Tags[0] != "摄影" {
		t.Fatalf("expected independent attempts: %v, %v", rows, err)
	}

	input.UpdatedAt = rows[0].EnrollmentUpdatedAt
	input.EnrollmentID = rejected.EnrollmentID
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err == nil {
		t.Fatal("using the new revision allowed approval of an archived attempt")
	}

	input.EnrollmentID = rows[0].EnrollmentID
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err != nil {
		t.Fatal(err)
	}

	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err != nil {
		t.Fatalf("identical retry: %v", err)
	}

	rows, err = service.EnrollmentUsers(ctx, "")
	if err != nil || len(rows) != 2 || rows[0].Status != enum.EnrollmentApproved || rows[1].Status != enum.EnrollmentRejected {
		t.Fatalf("reviewed enrollments: %v, %v", rows, err)
	}

	input.PortraitOnly = true
	input.UpdatedAt = rows[0].EnrollmentUpdatedAt
	input.Tags = []string{"时尚", "摄影"}
	if err := service.SaveEnrollmentReview(ctx, "admin", member.UserID, input); err != nil {
		t.Fatal(err)
	}

	current, err := service.Client.UserEnrollment.Get(ctx, input.EnrollmentID)
	if err != nil || current.Status != enum.EnrollmentApproved || len(current.Tags) != 2 {
		t.Fatalf("current enrollment portrait not updated: %v, %v", current, err)
	}

	updated, err := service.Client.UserEnrollment.Get(ctx, record.ID)
	if err != nil || updated.Status != enum.EnrollmentRejected || len(updated.Tags) != 1 || updated.Tags[0] != "摄影" {
		t.Fatalf("original portrait changed: %v, %v", updated, err)
	}

	notifications, err := service.Client.Notification.Query().
		Where(notification.UserIDEQ(member.UserID), notification.TitleEQ("报名审核结果")).
		Count(ctx)
	if err != nil || notifications != 2 {
		t.Fatalf("expected one notification per decision, got %d: %v", notifications, err)
	}

	edits, err := service.Client.AuditEvent.Query().
		Where(
			auditevent.ReferenceIDEQ(member.UserID),
			auditevent.ActionEQ(enum.AuditEnrollmentPortrait),
		).
		Count(ctx)
	if err != nil || edits != 1 {
		t.Fatalf("expected one portrait edit audit, got %d: %v", edits, err)
	}
}

func TestEnrollmentApplicantUsesItsOwnSnapshotAfterReapplication(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	member, err := service.DemoLogin(ctx, "enrollment-applicant-snapshot")
	if err != nil {
		t.Fatal(err)
	}

	setApplicant := func(nickname, name string) {
		t.Helper()
		if err := service.Client.User.UpdateOneID(member.UserID).SetNickname(nickname).Exec(ctx); err != nil {
			t.Fatal(err)
		}

		nameCipher, err := service.Encrypt(name)
		if err != nil {
			t.Fatal(err)
		}

		if err := service.Client.UserVerification.Create().
			SetUserID(member.UserID).
			SetNameCipher(nameCipher).
			SetVerifiedAt(service.Now()).
			OnConflictColumns(userverification.FieldUserID).
			UpdateNameCipher().
			Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	setApplicant("原昵称", "原姓名")
	terms, err := service.CurrentAgreement(ctx)
	if err != nil {
		t.Fatal(err)
	}

	input := enrollment.ApplyInput{
		Douyin:      "original-douyin",
		XHS:         "original-xhs",
		AgreementID: terms.ID,
	}
	if err := service.Apply(ctx, member.UserID, input); err != nil {
		t.Fatal(err)
	}

	rows, err := service.EnrollmentUsers(ctx, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("first enrollment: %d rows, %v", len(rows), err)
	}

	first := rows[0]
	if err := service.ReviewEnrollment(ctx, "admin", member.UserID, false, "补充资料", nil, first.EnrollmentUpdatedAt); err != nil {
		t.Fatal(err)
	}

	setApplicant("新昵称", "新姓名")
	input.PreviousEnrollmentID = first.EnrollmentID
	input.Douyin = "updated-douyin"
	if err := service.Apply(ctx, member.UserID, input); err != nil {
		t.Fatal(err)
	}

	rows, err = service.EnrollmentUsers(ctx, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("enrollment history: %d rows, %v", len(rows), err)
	}

	newer, older := rows[0], rows[1]
	if older.EnrollmentID != first.EnrollmentID || older.Name != first.Name || older.Nickname != first.Nickname ||
		older.Nickname != "原昵称" || older.Name != "原**" {
		t.Fatal("the older enrollment's applicant changed after reapplication")
	}

	if newer.Nickname != "新昵称" || newer.Name != "新**" {
		t.Fatal("the new enrollment did not keep the applicant at submission")
	}
}

func TestMarketingTagRenameKeepsTheMatchingKey(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	tag, err := service.CreateMarketingTag(ctx, "admin", "#时尚")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.RenameMarketingTag(ctx, "admin", tag.ID, "穿搭", tag.Name); err != nil {
		t.Fatal(err)
	}

	tags, err := service.MarketingTags(ctx)
	if err != nil || len(tags) != 1 || tags[0].Key != "时尚" || tags[0].Name != "穿搭" {
		t.Fatalf("renamed tag: %v, %v", tags, err)
	}

	if _, err := service.CreateMarketingTag(ctx, "admin", "穿搭"); err == nil {
		t.Fatal("duplicate tag name was accepted")
	}
}
