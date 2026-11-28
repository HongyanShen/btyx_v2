package main

import (
	"net/url"
	"regexp"
	"slices"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

type platformAccount struct {
	Text string
	URL  string
}

var barePlatformHost = regexp.MustCompile(`(?i)^(?:[\w-]+\.)+[a-z]+/`)
var platformAddress = regexp.MustCompile(`(?i)https?://[^\s<>"'，。！？、）】》]+`)

func playerAccount(value string, platform enum.Platform) platformAccount {
	account := platformAccount{Text: label(value, "—")}
	keyword := strings.TrimPrefix(strings.TrimSpace(value), "@")
	if keyword == "" {
		return account
	}

	hosts := []string{"douyin.com", "www.douyin.com", "v.douyin.com"}
	if platform == enum.PlatformXiaohongshu {
		hosts = []string{"xiaohongshu.com", "www.xiaohongshu.com", "xhslink.com"}
	}

	if barePlatformHost.MatchString(keyword) {
		keyword = "https://" + keyword
	}

	if address := platformAddress.FindString(keyword); address != "" {
		parsed, err := url.Parse(address)
		if err != nil || parsed.User != nil || !slices.Contains(hosts, strings.ToLower(parsed.Host)) {
			return account
		}

		path := label(parsed.EscapedPath(), "/")
		keyword = "https://" + strings.ToLower(parsed.Host) + path
	} else if strings.ContainsAny(keyword, ":/<>") {
		return account
	}

	escaped := strings.ReplaceAll(url.QueryEscape(keyword), "+", "%20")
	account.URL = "https://www.douyin.com/search/" + escaped + "?type=user"
	if platform == enum.PlatformXiaohongshu {
		account.URL = "https://www.xiaohongshu.com/search_result?keyword=" + escaped
	}

	return account
}
