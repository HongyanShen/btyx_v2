package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatpay"
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
	payments, err := wechatpay.New(ctx, cfg)
	if err != nil {
		return err
	}

	router := handler.NewRouter(services, payments)
	logging.From(ctx).Info().Str("address", cfg.MiniAddr).Str("role", string(enum.RoleUser)).Msg("API started")

	return httpx.Serve(ctx, cfg.MiniAddr, router)
}
