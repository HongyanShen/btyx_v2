package wechatmini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

const apiBase = "https://api.weixin.qq.com"

type PhoneClient struct {
	appID     string
	appSecret string
	client    *http.Client
}

func NewPhoneClient(appID, appSecret string) *PhoneClient {
	return &PhoneClient{
		appID:     appID,
		appSecret: appSecret,
		client: &http.Client{
			Timeout: 8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *PhoneClient) Number(ctx context.Context, code string) (string, error) {
	if c.appID == "" || c.appSecret == "" {
		return "", unavailable("微信手机号验证尚未配置")
	}

	tokenInput := struct {
		GrantType string `json:"grant_type"`
		AppID     string `json:"appid"`
		Secret    string `json:"secret"`
	}{
		GrantType: "client_credential",
		AppID:     c.appID,
		Secret:    c.appSecret,
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
	}

	// The stable-token API reuses the current token when force_refresh is omitted.
	if err := c.post(ctx, "/cgi-bin/stable_token", tokenInput, &token); err != nil || token.ErrCode != 0 || token.AccessToken == "" {
		return "", unavailable("微信手机号验证暂时不可用，请稍后重试")
	}

	phoneInput := struct {
		Code string `json:"code"`
	}{Code: code}
	var result struct {
		ErrCode   int `json:"errcode"`
		PhoneInfo struct {
			Number    string `json:"phoneNumber"`
			Watermark struct {
				AppID string `json:"appid"`
			} `json:"watermark"`
		} `json:"phone_info"`
	}

	endpoint := "/wxa/business/getuserphonenumber?access_token=" + url.QueryEscape(token.AccessToken)
	if err := c.post(ctx, endpoint, phoneInput, &result); err != nil {
		return "", unavailable("微信手机号验证暂时不可用，请稍后重试")
	}

	if result.ErrCode != 0 || result.PhoneInfo.Watermark.AppID != c.appID || result.PhoneInfo.Number == "" {
		return "", app.Invalid("未能验证微信手机号，请重新授权")
	}

	return result.PhoneInfo.Number, nil
}

func (c *PhoneClient) post(ctx context.Context, endpoint string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("unexpected WeChat response status")
	}

	return json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(output)
}

func unavailable(message string) error {
	return &app.Error{
		Status:  http.StatusServiceUnavailable,
		Code:    enum.ErrorUnavailable,
		Message: message,
	}
}
