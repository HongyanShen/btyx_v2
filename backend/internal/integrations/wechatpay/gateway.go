package wechatpay

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const transferEndpoint = "https://api.mch.weixin.qq.com/v3/fund-app/mch-transfer/transfer-bills"

type Gateway struct {
	client        *core.Client
	notifications *notify.Handler
	config        config.Payment
	appID         string
}

func New(ctx context.Context, cfg config.Config) (*Gateway, error) {
	if !cfg.Payment.Enabled {
		return nil, nil
	}

	if err := cfg.Payment.Validate(cfg.WechatAppID); err != nil {
		return nil, err
	}

	privateKey, err := utils.LoadPrivateKeyWithPath(cfg.Payment.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("无法加载微信商户私钥")
	}

	publicKey, err := utils.LoadPublicKeyWithPath(cfg.Payment.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("无法加载微信支付公钥")
	}

	client, err := core.NewClient(ctx,
		option.WithWechatPayPublicKeyAuthCipher(
			cfg.Payment.MerchantID, cfg.Payment.CertificateID,
			privateKey, cfg.Payment.PublicKeyID, publicKey,
		),
		option.WithHTTPClient(&http.Client{
			Timeout:   15 * time.Second,
			Transport: telemetry.Transport("wechat.pay"),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付官方 SDK 失败")
	}

	notifications, err := notify.NewRSANotifyHandler(
		cfg.Payment.APIv3Key,
		verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.Payment.PublicKeyID, *publicKey),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付通知处理器失败")
	}

	return &Gateway{
		client:        client,
		notifications: notifications,
		config:        cfg.Payment,
		appID:         cfg.WechatAppID,
	}, nil
}
