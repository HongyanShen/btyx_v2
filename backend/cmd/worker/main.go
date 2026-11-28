package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"baituber.cn/yanxuan/v2/cmd/worker/task"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatpay"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "worker"))
	cfg, err := config.Load(config.Path())
	if err != nil {
		logging.Error(ctx, err).Msg("读取配置失败")
		os.Exit(1)
	}

	logger, closer, err := logging.Open(cfg.Logging, "worker")
	if err != nil {
		logging.Error(ctx, err).Msg("打开日志文件失败")
		os.Exit(1)
	}

	defer closer.Close()
	ctx = logging.WithLogger(ctx, logger)
	closeTrace, err := telemetry.Open(ctx, cfg, "worker")
	if err != nil {
		logging.Error(ctx, err).Msg("初始化链路追踪失败")
		os.Exit(1)
	}

	defer closeTrace()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	services, err := business.New(ctx, cfg)
	if err != nil {
		logging.Error(ctx, err).Msg("数据库连接失败")
		_ = closer.Close()
		os.Exit(1)
	}

	defer services.Close()
	payments, err := wechatpay.New(ctx, cfg)
	if err != nil {
		logging.Error(ctx, err).Msg("初始化微信支付 SDK 失败")
		_ = closer.Close()
		os.Exit(1)
	}

	var gateway transfer.Gateway
	if payments != nil {
		gateway = payments
	}

	notices, err := task.StartNotices(ctx, services)
	if err != nil {
		logging.Error(ctx, err).Msg("启动通知队列失败")
		_ = closer.Close()
		os.Exit(1)
	}

	logging.Info(ctx).Msg("worker started")
	defer notices.Stop(context.Background())
	runner := task.New(services, gateway)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickContext := logging.NewRequest(ctx)
			if err := runner.Tick(tickContext); err != nil {
				logging.Error(tickContext, err).Msg("后台任务处理失败，等待重试")
			}
		}
	}
}
