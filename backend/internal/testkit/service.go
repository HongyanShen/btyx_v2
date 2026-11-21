package testkit

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/database"
)

func Service(t testing.TB, root string) *business.Service {
	t.Helper()

	cfg, err := config.Load(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}

	cfg.Schema = "btyx_test_" + strings.ToLower(business.NewID())
	cfg.Demo = true

	ctx := context.Background()
	if err := database.Migrate(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	service, err := business.New(ctx, cfg)
	if err != nil {
		_ = database.DropTest(ctx, cfg)
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = service.Close()
		if err := database.DropTest(context.Background(), cfg); err != nil {
			t.Error(err)
		}
	})

	return service
}
