package main

import (
	"context"
	"os"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "miniapi"))
	cfg, err := config.Load(config.Path())
	if err != nil {
		logging.Error(ctx, err).Msg("读取配置失败")
		os.Exit(1)
	}

	logger, closer, err := logging.Open(cfg.Logging, "miniapi")
	if err != nil {
		logging.Error(ctx, err).Msg("打开日志文件失败")
		os.Exit(1)
	}

	defer closer.Close()
	ctx = logging.WithLogger(ctx, logger)
	if err := serve(ctx, cfg); err != nil {
		logging.Error(ctx, err).Msg("小程序 API 启动失败")
		_ = closer.Close()
		os.Exit(1)
	}
}
