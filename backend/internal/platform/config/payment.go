package config

import (
	"fmt"
	"net/url"
)

type SceneReport struct {
	Type    string `json:"info_type"`
	Content string `json:"info_content"`
}

type Payment struct {
	Enabled        bool
	MerchantID     string
	CertificateID  string
	PrivateKeyPath string
	PublicKeyID    string
	PublicKeyPath  string
	APIv3Key       string
	NotifyURL      string
	SceneID        string
	SceneReports   []SceneReport
}

func (payment Payment) Validate(appID string) error {
	if !payment.Enabled {
		return nil
	}

	if appID == "" || payment.MerchantID == "" || payment.CertificateID == "" ||
		payment.PrivateKeyPath == "" || payment.PublicKeyID == "" || payment.PublicKeyPath == "" ||
		len(payment.APIv3Key) != 32 || payment.SceneID == "" || len(payment.SceneReports) == 0 {
		return fmt.Errorf("微信支付配置不完整，请检查商户、密钥、公钥与转账场景配置")
	}

	notifyURL, err := url.Parse(payment.NotifyURL)
	if err != nil || notifyURL.Scheme != "https" || notifyURL.Host == "" || notifyURL.RawQuery != "" {
		return fmt.Errorf("微信支付回调地址必须是无查询参数的 HTTPS 地址")
	}

	for _, report := range payment.SceneReports {
		if report.Type == "" || report.Content == "" {
			return fmt.Errorf("转账场景报备信息不能为空")
		}
	}

	return nil
}
