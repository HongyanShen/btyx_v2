package main

import (
	"context"
	"fmt"
	"os"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

func main() {
	ctx := logging.WithLogger(context.Background(), logging.New(os.Stderr, "seed"))
	cfg, err := config.Load(config.Path())
	if err != nil {
		logging.Error(ctx, err).Msg("读取配置失败")
		os.Exit(1)
	}

	logger, closer, err := logging.Open(cfg.Logging, "seed")
	if err != nil {
		logging.Error(ctx, err).Msg("打开日志文件失败")
		os.Exit(1)
	}

	defer closer.Close()
	ctx = logging.WithLogger(ctx, logger)
	if err := seed(ctx, cfg); err != nil {
		logging.Error(ctx, err).Msg("本地示例数据初始化失败")
		_ = closer.Close()
		os.Exit(1)
	}
}

func seed(ctx context.Context, cfg config.Config) error {
	if !cfg.Demo || cfg.Payment.Enabled {
		return fmt.Errorf("示例数据仅允许 DEMO_ENABLED=true 且支付关闭的本地环境")
	}

	services, err := business.New(ctx, cfg)
	if err != nil {
		return err
	}

	defer services.Close()
	count, err := services.Client.Agreement.Query().Count(ctx)
	if err != nil {
		return err
	}

	if count == 0 {
		if err := services.PublishAgreement(ctx, "admin", "本地演示合作协议（待运营替换）", "本协议仅供本地验收，不作为正式合作协议。\n请在发布真实任务前由运营发布正式协议。\n任务要求与价格以领取或派发时的版本为准；按平台审核，周结算后可以按余额申请提现。"); err != nil {
			return err
		}
	}

	count, err = services.Client.Task.Query().Count(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		logging.Info(ctx).Msg("已有任务，保留现有内容，不重复添加示例")

		return nil
	}

	for _, input := range samples() {
		contentRecord, err := services.CreateContent(ctx, "admin", content.Input{
			Title:   input.Title,
			Body:    input.Body,
			Project: input.Project,
			Tags:    input.Tags,
		})
		if err != nil {
			return err
		}

		input.ContentID = contentRecord.ID
		if _, err := services.CreateTask(ctx, "admin", input); err != nil {
			return err
		}
	}

	logging.Info(ctx).Msg("已创建本地示例协议、内容及任务；没有创建余额、付款或审核结果")
	return nil
}

func samples() []business.TaskInput {
	return []business.TaskInput{
		{
			Title:     "家居好物 · 营销分发示例",
			Body:      "【本地演示任务】\n核对发布文案与平台要求后发布，按平台提交作品链接，并补充播放数据截图。\n正式任务请先在内容池上传素材，再发布对应版本。",
			Project:   "家居生活",
			PriceFen:  "750",
			Platforms: []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu},
			Tags:      []string{"家居", "生活"},
		},
		{
			Title:     "美妆日常 · 营销分发示例",
			Body:      "【本地演示任务】\n按任务要求发布抖音作品，提交可访问的 HTTPS 链接和播放数据截图，等待平台审核。",
			Project:   "日常美妆",
			PriceFen:  "900",
			Platforms: []enum.Platform{enum.PlatformDouyin},
			Tags:      []string{"美妆", "时尚"},
		},
		{
			Title:     "生活方式 · 营销分发示例",
			Body:      "【本地演示任务】\n在小红书发布符合要求的作品，保留作品链接与数据截图。审核通过后等待周结算入账。",
			Project:   "品质生活",
			PriceFen:  "800",
			Platforms: []enum.Platform{enum.PlatformXiaohongshu},
			Tags:      []string{"生活", "好物"},
		},
	}
}
