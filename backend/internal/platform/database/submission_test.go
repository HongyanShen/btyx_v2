package database_test

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestSubmissionMigrationRemovesOnlyLegacyWorkLinkIndex(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	_, err := service.DB.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS submission_platform_work_key ON submissions (platform, work_key);
		CREATE INDEX submission_custom_status_idx ON submissions (status);
	`)
	if err != nil {
		t.Fatal(err)
	}

	const link = "https://xhslink.cn/o/shared-work"
	const workKey = "xhslink.cn/o/shared-work"
	legacy, err := service.Client.Submission.Create().
		SetAssignmentID("original-assignment").
		SetPlatform(enum.PlatformXiaohongshu).
		SetURL(link).
		SetWorkKey(workKey).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	preserved, err := service.Client.Submission.Get(ctx, legacy.ID)
	if err != nil || preserved.URL != link || preserved.WorkKey != workKey {
		t.Fatalf("legacy submission changed: %v, %v", preserved, err)
	}

	var legacyIndexExists, customIndexExists bool
	err = service.DB.QueryRowContext(ctx, `
		SELECT to_regclass('submission_platform_work_key') IS NOT NULL,
		       to_regclass('submission_custom_status_idx') IS NOT NULL
	`).Scan(&legacyIndexExists, &customIndexExists)
	if err != nil || legacyIndexExists || !customIndexExists {
		t.Fatalf("unexpected index state: legacy=%t custom=%t error=%v", legacyIndexExists, customIndexExists, err)
	}

	err = service.Client.Submission.Create().
		SetAssignmentID("another-assignment").
		SetPlatform(enum.PlatformXiaohongshu).
		SetURL(link).
		SetWorkKey(workKey).
		Exec(ctx)
	if err != nil {
		t.Fatalf("legacy work link still blocks other assignments: %v", err)
	}

	err = service.Client.Submission.Create().
		SetAssignmentID("original-assignment").
		SetPlatform(enum.PlatformXiaohongshu).
		SetURL("https://xhslink.cn/o/another-work").
		SetWorkKey("xhslink.cn/o/another-work").
		Exec(ctx)
	if !ent.IsConstraintError(err) {
		t.Fatalf("database must forbid two submissions for the same assignment platform: %v", err)
	}
}
