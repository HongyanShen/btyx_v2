package database_test

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestEnrollmentMigrationPreservesRecordsAndReplacesLegacyUniqueConstraint(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	member, err := service.DemoLogin(ctx, "legacy-enrollment")
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentRejected).
		SetReason("旧驳回记录").
		SetTags([]string{"摄影"}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.DB.ExecContext(ctx, `
		DROP INDEX user_enrollment_current_user;
		DROP INDEX userenrollment_user_id_created_at;
		ALTER TABLE user_enrollments DROP COLUMN is_current, DROP COLUMN douyin, DROP COLUMN xhs, DROP COLUMN applicant_nickname;
		ALTER TABLE user_enrollments ADD UNIQUE (user_id);
		CREATE INDEX enrollment_custom_reason_idx ON user_enrollments (reason);
	`)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	preserved, err := service.Client.UserEnrollment.Get(ctx, legacy.ID)
	if err != nil || !preserved.IsCurrent || preserved.Status != enum.EnrollmentRejected ||
		preserved.Reason != "旧驳回记录" || len(preserved.Tags) != 1 || preserved.Tags[0] != "摄影" ||
		preserved.ApplicantNickname != "" {
		t.Fatalf("legacy enrollment changed: %v, %v", preserved, err)
	}

	var customIndexExists bool
	err = service.DB.QueryRowContext(ctx, `
		SELECT to_regclass('enrollment_custom_reason_idx') IS NOT NULL
	`).Scan(&customIndexExists)
	if err != nil || !customIndexExists {
		t.Fatalf("unrelated custom index removed: %v", err)
	}

	err = service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentPending).
		Exec(ctx)
	if !ent.IsConstraintError(err) {
		t.Fatalf("database must forbid two current attempts: %v", err)
	}

	if err := service.Client.UserEnrollment.UpdateOne(preserved).SetIsCurrent(false).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	err = service.Client.UserEnrollment.Create().
		SetUserID(member.UserID).
		SetStatus(enum.EnrollmentPending).
		Exec(ctx)
	if err != nil {
		t.Fatalf("legacy unique constraint still blocks new attempts: %v", err)
	}
}
