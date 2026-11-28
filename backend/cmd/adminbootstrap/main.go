package main

import (
	"context"
	"os"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "adminbootstrap"))
	if !bootstrap(ctx) {
		os.Exit(1)
	}
}

func bootstrap(ctx context.Context) bool {
	cfg, err := config.Load(config.Path())
	if err != nil {
		logging.Error(ctx, err).Msg("读取配置失败")
		return false
	}

	if cfg.BootstrapAdminUsername == "" || cfg.BootstrapAdminPassword == "" {
		logging.Error(ctx, nil).Msg("请在本地配置填写 BOOTSTRAP_ADMIN_USERNAME 和 BOOTSTRAP_ADMIN_PASSWORD")
		return false
	}

	runtime, err := app.Open(ctx, cfg)
	if err != nil {
		logging.Error(ctx, err).Msg("连接服务失败")
		return false
	}

	defer runtime.Close()
	service := &identity.Service{Runtime: runtime}

	if err := service.BootstrapAdmin(ctx, cfg.BootstrapAdminUsername, cfg.BootstrapAdminPassword); err != nil {
		logging.Error(ctx, err).Msg("超级管理员初始化失败，请确认已完成迁移、账号尚未初始化且凭据符合格式要求")
		return false
	}

	logging.Info(ctx).Msg("超级管理员已初始化")
	return true
}
