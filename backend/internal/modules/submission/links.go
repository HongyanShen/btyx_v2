package submission

import (
	"net/url"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/internal/platform/app"
)

func workKey(platform enum.Platform, raw string) (string, error) {
	return WorkKey(platform, raw)
}

func WorkKey(platform enum.Platform, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return "", app.Invalid("请提供 HTTPS 平台作品链接")
	}

	host := strings.ToLower(u.Hostname())
	isDouyin := platform == enum.PlatformDouyin &&
		(host == "v.douyin.com" || host == "www.douyin.com")
	isXiaohongshu := platform == enum.PlatformXiaohongshu &&
		(host == "xhslink.cn" || host == "xhslink.com" || host == "www.xiaohongshu.com")
	if (!isDouyin && !isXiaohongshu) || u.Path == "" || u.Path == "/" {
		return "", app.Invalid("链接与发布平台不一致")
	}

	u.RawQuery = ""
	u.Fragment = ""
	return host + strings.TrimRight(u.Path, "/"), nil
}
