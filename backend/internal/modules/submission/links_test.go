package submission

import (
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestWorkKeyXiaohongshuLinks(t *testing.T) {
	for _, test := range []struct {
		name     string
		platform enum.Platform
		url      string
		want     string
	}{
		{
			name:     "cn short link",
			platform: enum.PlatformXiaohongshu,
			url:      "https://xhslink.cn/o/5IKcSj0r69I/?source=share#note",
			want:     "xhslink.cn/o/5IKcSj0r69I",
		},
		{
			name:     "com short link",
			platform: enum.PlatformXiaohongshu,
			url:      "https://xhslink.com/a/efgh",
			want:     "xhslink.com/a/efgh",
		},
		{
			name:     "note link",
			platform: enum.PlatformXiaohongshu,
			url:      "https://www.xiaohongshu.com/explore/abc123",
			want:     "www.xiaohongshu.com/explore/abc123",
		},
		{
			name:     "unlisted host",
			platform: enum.PlatformXiaohongshu,
			url:      "https://xhslink.cn.example.com/o/abc123",
		},
		{
			name:     "wrong platform",
			platform: enum.PlatformDouyin,
			url:      "https://xhslink.cn/o/abc123",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := workKey(test.platform, test.url)
			if got != test.want || (err == nil) != (test.want != "") {
				t.Fatalf("workKey = %q, %v, want %q", got, err, test.want)
			}
		})
	}
}
