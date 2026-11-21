package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type ObjectStorage struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	Prefix    string
}

func (storage ObjectStorage) Configured() bool {
	return storage.Bucket != "" || storage.Region != "" || storage.Endpoint != "" || storage.AccessKey != "" || storage.SecretKey != ""
}

func (storage ObjectStorage) Validate() error {
	if !storage.Configured() {
		return nil
	}

	if storage.Bucket == "" || storage.Region == "" || storage.Endpoint == "" || storage.AccessKey == "" || storage.SecretKey == "" {
		return fmt.Errorf("TOS 配置不完整，需要 Bucket、Region、Endpoint 及专用 AK/SK")
	}

	endpoint, err := url.Parse(storage.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != "" && endpoint.Path != "/" {
		return fmt.Errorf("TOS_ENDPOINT 必须是地域 HTTPS Endpoint，不包含桶名、路径或查询参数")
	}

	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`).MatchString(storage.Bucket) {
		return fmt.Errorf("TOS_BUCKET 格式无效")
	}

	if storage.Prefix == "" || strings.HasPrefix(storage.Prefix, "/") || !strings.HasSuffix(storage.Prefix, "/") ||
		!regexp.MustCompile(`^[a-zA-Z0-9_/-]+/$`).MatchString(storage.Prefix) {
		return fmt.Errorf("TOS_OBJECT_PREFIX 必须为非空相对目录，以 / 结尾，仅包含字母、数字、_、-、/")
	}

	return nil
}
