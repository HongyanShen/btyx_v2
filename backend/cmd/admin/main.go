package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "admin"))
	path := config.Path()
	cfg, err := config.Load(path)
	if err != nil {
		logging.Error(ctx, err).Msg("读取配置失败")
		os.Exit(1)
	}

	logger, closer, err := logging.Open(cfg.Logging, "admin")
	if err != nil {
		logging.Error(ctx, err).Msg("打开日志文件失败")
		os.Exit(1)
	}

	ctx = logging.WithLogger(ctx, logger)
	closeTrace, err := telemetry.Open(ctx, cfg, "admin")
	if err != nil {
		logging.Error(ctx, err).Msg("初始化链路追踪失败")
		os.Exit(1)
	}

	defer closeTrace()
	err = serve(ctx, cfg)
	if err != nil {
		logging.Error(ctx, err).Msg("HTMX 管理后台启动失败")
	}

	_ = closer.Close()
	if err != nil {
		os.Exit(1)
	}
}

func serve(ctx context.Context, cfg config.Config) error {
	host, _, err := net.SplitHostPort(cfg.AdminWebAddr)
	if err != nil {
		return fmt.Errorf("ADMIN_WEB_ADDR 格式无效: %w", err)
	}

	ip := net.ParseIP(host)
	if !cfg.AdminWebSecureCookie && host != "localhost" && !ip.IsLoopback() {
		return fmt.Errorf("非回环地址需要启用 ADMIN_WEB_SECURE_COOKIE 并通过 HTTPS 访问")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	services, err := business.New(ctx, cfg)
	if err != nil {
		return err
	}

	defer services.Close()

	router, err := newAdminRouter(services, cfg.AdminWebSecureCookie)
	if err != nil {
		return err
	}

	logging.Info(ctx).Str("address", cfg.AdminWebAddr).Msg("HTMX admin started")

	return httpx.Serve(ctx, cfg.AdminWebAddr, router)
}
