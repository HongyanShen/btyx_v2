package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func serve(ctx context.Context, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	services, err := business.New(ctx, cfg)
	if err != nil {
		return err
	}

	defer services.Close()

	router := handler.NewRouter(services)
	logging.From(ctx).Info().Str("address", cfg.AdminAddr).Str("role", string(enum.RoleAdmin)).Msg("API started")

	return httpx.Serve(ctx, cfg.AdminAddr, router)
}
