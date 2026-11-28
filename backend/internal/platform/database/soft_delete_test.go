package database_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	entcontent "baituber.cn/yanxuan/v2/ent/content"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestEntDeletesMarkRowsAndHonorTransaction(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	initialUpdate := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	task, err := service.Client.Task.Create().
		SetDisplayNumber(1).
		SetCreatorID("admin").
		SetUpdatedAt(initialUpdate).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := service.Client.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer tx.Rollback()
	if err := tx.Task.DeleteOneID(task.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Task.Get(ctx, task.ID); !ent.IsNotFound(err) {
		t.Fatalf("task remains visible inside the deleting transaction: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	rolledBack, err := service.Client.Task.Get(ctx, task.ID)
	if err != nil || !rolledBack.UpdatedAt.Equal(initialUpdate) {
		t.Fatalf("rollback did not restore visibility and update time: %v, %v", rolledBack, err)
	}

	if err := service.Client.Task.DeleteOneID(task.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	var taskUpdatedAt time.Time
	err = service.DB.QueryRowContext(ctx, `SELECT updated_at FROM tasks WHERE id = $1 AND deleted`, task.ID).Scan(&taskUpdatedAt)
	if err != nil || !taskUpdatedAt.After(initialUpdate) {
		t.Errorf("single delete did not retain the task and refresh its update time: %v, %v", taskUpdatedAt, err)
	}

	if err := service.Client.Task.DeleteOneID(task.ID).Exec(ctx); !ent.IsNotFound(err) {
		t.Errorf("repeat single delete should affect zero rows: %v", err)
	}

	if err := service.Client.Task.UpdateOneID(task.ID).SetOnline(false).Exec(ctx); !ent.IsNotFound(err) {
		t.Errorf("update of deleted task: %v", err)
	}

	var retryUpdatedAt time.Time
	err = service.DB.QueryRowContext(ctx, `SELECT updated_at FROM tasks WHERE id = $1`, task.ID).Scan(&retryUpdatedAt)
	if err != nil || !retryUpdatedAt.Equal(taskUpdatedAt) {
		t.Errorf("retry or rejected edit changed the task update time: %v, %v", retryUpdatedAt, err)
	}

	material, err := service.Client.Content.Create().
		SetTitle("旧素材").
		SetBody("内容").
		SetCreatorID("admin").
		SetUpdatedAt(initialUpdate).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	removed, err := service.Client.Content.Delete().Where(entcontent.IDEQ(material.ID)).Exec(ctx)
	if err != nil || removed != 1 {
		t.Fatalf("bulk delete: count %d, %v", removed, err)
	}

	var contentUpdatedAt time.Time
	err = service.DB.QueryRowContext(ctx, `SELECT updated_at FROM contents WHERE id = $1 AND deleted`, material.ID).Scan(&contentUpdatedAt)
	if err != nil || !contentUpdatedAt.After(initialUpdate) {
		t.Errorf("bulk delete did not retain the content and refresh its update time: %v, %v", contentUpdatedAt, err)
	}

	removed, err = service.Client.Content.Delete().Where(entcontent.IDEQ(material.ID)).Exec(ctx)
	if err != nil || removed != 0 {
		t.Errorf("repeat bulk delete: count %d, %v", removed, err)
	}

	err = service.DB.QueryRowContext(ctx, `SELECT updated_at FROM contents WHERE id = $1`, material.ID).Scan(&retryUpdatedAt)
	if err != nil || !retryUpdatedAt.Equal(contentUpdatedAt) {
		t.Errorf("repeat bulk delete changed the content update time: %v, %v", retryUpdatedAt, err)
	}
}

func TestSoftDeletedRowsAreHiddenFromEntQueries(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	client := service.Client
	active, err := service.DemoLogin(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := service.DemoLogin(ctx, "removed")
	if err != nil {
		t.Fatal(err)
	}

	err = client.User.DeleteOneID(deleted.UserID).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = client.Content.Create().
		SetTitle("旧素材").
		SetBody("内容").
		SetCreatorID("admin").
		SetDeleted(true).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	deletedTask, err := client.Task.Create().
		SetDisplayNumber(1).
		SetCreatorID("admin").
		SetDeleted(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = client.MarketingTag.Create().
		SetKey("removed").
		SetName("旧标签").
		SetDeleted(true).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name  string
		count func(context.Context) (int, error)
		want  int
	}{
		{
			name:  "users",
			count: client.User.Query().Count,
			want:  1,
		},
		{
			name:  "content",
			count: client.Content.Query().Count,
		},
		{
			name:  "tasks",
			count: client.Task.Query().Count,
		},
		{
			name:  "tags",
			count: client.MarketingTag.Query().Count,
		},
		{
			name:  "traversal from users",
			count: client.User.Query().QueryWechatIdentity().Count,
			want:  1,
		},
	}
	for _, check := range checks {
		got, err := check.count(ctx)
		if err != nil || got != check.want {
			t.Errorf("%s count = %d, %v; want %d", check.name, got, err, check.want)
		}
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer tx.Rollback()
	if _, err := tx.User.Get(ctx, deleted.UserID); !ent.IsNotFound(err) {
		t.Errorf("transactional lookup of deleted user: %v", err)
	}

	if _, err := tx.User.Get(ctx, active.UserID); err != nil {
		t.Errorf("transactional lookup of active user: %v", err)
	}

	identities, err := client.WechatIdentity.Query().WithUser().All(ctx)
	if err != nil || len(identities) != 2 {
		t.Fatalf("identities: count %d, %v", len(identities), err)
	}

	for _, identity := range identities {
		wantUser := identity.UserID == active.UserID
		if (identity.Edges.User != nil) != wantUser {
			t.Errorf("unexpected user visibility for identity %s", identity.ID)
		}
	}

	_, err = service.DemoLogin(ctx, "removed")
	var appError *app.Error
	if !errors.As(err, &appError) || appError.Status != 403 || appError.Message != "账号不可用" {
		t.Errorf("login for deleted user: %v", err)
	}

	if _, err := service.Authenticate(ctx, deleted.Token); !errors.As(err, &appError) || appError.Status != 401 {
		t.Errorf("existing session of deleted user: %v", err)
	}

	if _, err := service.Authenticate(ctx, active.Token); err != nil {
		t.Errorf("session of active user: %v", err)
	}

	err = service.SetOnline(ctx, "admin", deletedTask.ID, false)
	if !errors.As(err, &appError) || appError.Status != 404 {
		t.Errorf("visibility update for deleted task: %v", err)
	}
}

func TestSoftDeletedMarketingTagIsNotRestored(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	tag, err := service.CreateMarketingTag(ctx, "admin", "摄影")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.DeleteMarketingTag(ctx, "admin", tag.ID, tag.UpdatedAt); err != nil {
		t.Fatal(err)
	}

	if err := service.DeleteMarketingTag(ctx, "admin", tag.ID, tag.UpdatedAt); err != nil {
		t.Fatalf("repeated deletion: %v", err)
	}

	_, err = service.CreateContent(ctx, "admin", content.Input{
		Title: "新素材",
		Body:  "发布要求",
		Tags:  []string{tag.Key},
	})
	var appError *app.Error
	if !errors.As(err, &appError) || appError.Status != 409 {
		t.Errorf("binding a deleted tag: %v", err)
	}

	replacement, err := service.CreateMarketingTag(ctx, "admin", tag.Name)
	if err != nil || replacement.ID == tag.ID || replacement.Key == tag.Key {
		t.Fatalf("replacement tag: %v, %v", replacement, err)
	}

	if err := service.DeleteMarketingTag(ctx, "admin", tag.ID, tag.UpdatedAt); err != nil {
		t.Fatalf("repeated deletion after adding a same-name tag: %v", err)
	}

	visible, err := service.Client.MarketingTag.Query().Only(ctx)
	if err != nil || visible.ID != replacement.ID {
		t.Fatalf("visible tag: %v, %v", visible, err)
	}
}

func TestMarketingTagMigrationPreservesRetiredKeys(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	oldTag, err := service.Client.MarketingTag.Create().
		SetKey("历史键").
		SetName("历史标签").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.DB.ExecContext(ctx, `
		DROP INDEX marketing_tag_active_name;
		DROP INDEX marketing_tag_active_key;
		ALTER TABLE marketing_tags DROP COLUMN deleted, ADD COLUMN deleted_at bigint NOT NULL DEFAULT 0;
		ALTER TABLE marketing_tags ADD UNIQUE (name);
		ALTER TABLE marketing_tags ADD UNIQUE (key);
		CREATE INDEX marketing_tags_custom_created_at_idx ON marketing_tags (created_at);
		DROP TABLE marketing_tag_deletions;
	`)
	if err != nil {
		t.Fatal(err)
	}

	deletedAt := time.Date(2026, time.November, 17, 5, 10, 3, 123000000, time.UTC)
	_, err = service.DB.ExecContext(ctx, `UPDATE marketing_tags SET deleted_at = $1 WHERE id = $2`, deletedAt.UnixMilli(), oldTag.ID)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := service.Client.MarketingTag.Get(ctx, oldTag.ID); !ent.IsNotFound(err) {
		t.Errorf("migrated tag should be hidden: %v", err)
	}

	deletion, err := service.Client.MarketingTagDeletion.Get(ctx, oldTag.ID)
	if err != nil || deletion.Key != oldTag.Key || !deletion.CreatedAt.Equal(deletedAt) {
		t.Fatalf("migrated tag deletion: %v, %v", deletion, err)
	}

	var indexDefinition string
	err = service.DB.QueryRowContext(ctx, `
		SELECT pg_get_indexdef('marketing_tag_active_name'::regclass)
	`).Scan(&indexDefinition)
	if err != nil || !strings.Contains(indexDefinition, "(name) WHERE (NOT deleted)") {
		t.Errorf("active tag name index: %s, %v", indexDefinition, err)
	}

	err = service.DB.QueryRowContext(ctx, `
		SELECT coalesce(pg_get_indexdef(to_regclass('marketing_tag_active_key')), '')
	`).Scan(&indexDefinition)
	if err != nil || !strings.Contains(indexDefinition, "(key) WHERE (NOT deleted)") {
		t.Errorf("active tag key index: %s, %v", indexDefinition, err)
	}

	newTag, err := service.CreateMarketingTag(ctx, "admin", oldTag.Name)
	if err != nil {
		t.Fatalf("create after legacy unique constraint migration: %v", err)
	}

	err = service.Client.MarketingTag.Create().
		SetKey("另一个匹配键").
		SetName(oldTag.Name).
		Exec(ctx)
	if !ent.IsConstraintError(err) {
		t.Errorf("duplicate active name: %v", err)
	}

	err = service.Client.MarketingTag.Create().
		SetKey(newTag.Key).
		SetName("不同名称").
		Exec(ctx)
	if !ent.IsConstraintError(err) {
		t.Errorf("duplicate active key: %v", err)
	}

	err = service.Client.MarketingTag.Create().
		SetKey("另一个已删匹配键").
		SetName(oldTag.Name).
		SetDeleted(true).
		Exec(ctx)
	if err != nil {
		t.Errorf("historical duplicate name: %v", err)
	}

	_, err = service.CreateContent(ctx, "admin", content.Input{
		Title: "新素材",
		Body:  "发布要求",
		Tags:  []string{oldTag.Key},
	})
	var appError *app.Error
	if !errors.As(err, &appError) || appError.Status != 409 {
		t.Errorf("binding a legacy retired key: %v", err)
	}

	var keptCustomIndex bool
	var removedLegacyIndexes bool
	err = service.DB.QueryRowContext(ctx, `
		SELECT to_regclass('marketing_tags_custom_created_at_idx') IS NOT NULL,
		       to_regclass('marketing_tags_name_key') IS NULL AND to_regclass('marketing_tags_key_key') IS NULL
	`).Scan(&keptCustomIndex, &removedLegacyIndexes)
	if err != nil || !keptCustomIndex || !removedLegacyIndexes {
		t.Fatalf("legacy index migration: kept custom index = %t, removed legacy indexes = %t, %v", keptCustomIndex, removedLegacyIndexes, err)
	}
}

func TestBooleanSoftDeleteMigrationFromCompositePartialIndexes(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := context.Background()
	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE users DROP COLUMN deleted, ADD COLUMN deleted_at bigint NOT NULL DEFAULT 0;
		ALTER TABLE contents DROP COLUMN deleted, ADD COLUMN deleted_at bigint NOT NULL DEFAULT 0;
		ALTER TABLE tasks DROP COLUMN display_number, DROP COLUMN deleted, ADD COLUMN deleted_at bigint NOT NULL DEFAULT 0;
		ALTER TABLE marketing_tags DROP COLUMN deleted, ADD COLUMN deleted_at bigint NOT NULL DEFAULT 0;

		CREATE UNIQUE INDEX marketing_tag_active_name ON marketing_tags (name, deleted_at) WHERE deleted_at = 0;
		CREATE UNIQUE INDEX marketing_tag_active_key ON marketing_tags (key, deleted_at) WHERE deleted_at = 0;

		INSERT INTO users (id, created_at, updated_at, deleted_at) VALUES ('legacy-user', now(), now(), 1730000000123);
		INSERT INTO contents (id, created_at, updated_at, title, body, creator_id, deleted_at)
		VALUES ('legacy-content', now(), now(), '旧素材', '内容', 'admin', 1730000000123);
		INSERT INTO tasks (id, created_at, updated_at, creator_id, deleted_at)
		VALUES ('legacy-task', now(), now(), 'admin', 1730000000123);
		INSERT INTO marketing_tags (id, created_at, updated_at, key, name, deleted_at)
		VALUES ('legacy-tag', now(), now(), '旧匹配键', '同名标签', 1730000000123),
		       ('active-tag', now(), now(), '有效匹配键', '同名标签', 0);
	`)
	if err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(ctx, service.Config); err != nil {
		t.Fatal(err)
	}

	var archivedRows int
	var legacyColumns int
	err = service.DB.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM users WHERE id = 'legacy-user' AND deleted) +
		       (SELECT count(*) FROM contents WHERE id = 'legacy-content' AND deleted) +
		       (SELECT count(*) FROM tasks WHERE id = 'legacy-task' AND deleted) +
		       (SELECT count(*) FROM marketing_tags WHERE id = 'legacy-tag' AND deleted),
		       (SELECT count(*) FROM information_schema.columns
		        WHERE table_schema = current_schema()
		          AND table_name IN ('users', 'contents', 'tasks', 'marketing_tags')
		          AND column_name = 'deleted_at')
	`).Scan(&archivedRows, &legacyColumns)
	if err != nil || archivedRows != 4 || legacyColumns != 0 {
		t.Fatalf("archived rows = %d, obsolete columns = %d, %v", archivedRows, legacyColumns, err)
	}

	tag, err := service.Client.MarketingTag.Query().Only(ctx)
	if err != nil || tag.ID != "active-tag" {
		t.Fatalf("visible tag after migration: %v, %v", tag, err)
	}
}
