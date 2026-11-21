package identity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func (s *Service) LoginWechat(ctx context.Context, code string) (LoginResult, error) {
	if strings.TrimSpace(code) == "" || len(code) > 512 {
		return LoginResult{}, app.Invalid("微信登录凭据无效，请重新登录")
	}

	if s.Config.WechatAppID == "" || s.Config.WechatAppSecret == "" {
		return LoginResult{}, &app.Error{
			Status:  http.StatusServiceUnavailable,
			Code:    enum.ErrorUnavailable,
			Message: "微信登录尚未配置",
		}
	}

	parameters := url.Values{
		"appid":      {s.Config.WechatAppID},
		"secret":     {s.Config.WechatAppSecret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?" + parameters.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LoginResult{}, app.Invalid("无法发起微信登录")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return LoginResult{}, app.Conflict("微信服务暂时不可用，请稍后重试")
	}

	defer response.Body.Close()
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result); err != nil {
		return LoginResult{}, app.Conflict("微信服务返回异常，请重试")
	}

	if response.StatusCode != http.StatusOK || result.ErrCode != 0 || result.OpenID == "" {
		return LoginResult{}, app.Invalid("微信登录已失效，请重新登录")
	}

	return s.loginOpenID(ctx, result.OpenID)
}
