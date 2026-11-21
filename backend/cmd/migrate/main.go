package main

import (
	"context"
	"os"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "migrate"))
	cfg, err := config.Load(config.Path())
	if err != nil {
		logging.Error(ctx, err).Msg("读取迁移配置失败")
		os.Exit(1)
	}

	logger, closer, err := logging.Open(cfg.Logging, "migrate")
	if err != nil {
		logging.Error(ctx, err).Msg("打开日志文件失败")
		os.Exit(1)
	}

	defer closer.Close()
	ctx = logging.WithLogger(ctx, logger)

	if err := database.Migrate(ctx, cfg); err != nil {
		logging.Error(ctx, err).Msg("Ent migration failed")
		_ = closer.Close()
		os.Exit(1)
	}

	logging.From(ctx).Info().Str("schema", cfg.Schema).Msg("Ent migration completed")
}
