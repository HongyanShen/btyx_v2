package config

import (
	"fmt"
	"strings"
)

type Logging struct {
	Directory  string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

func (settings Logging) Validate() error {
	if strings.TrimSpace(settings.Directory) == "" {
		return fmt.Errorf("LOG_DIR 不能为空")
	}

	if settings.MaxSizeMB < 1 || settings.MaxBackups < 1 || settings.MaxAgeDays < 1 {
		return fmt.Errorf("日志文件大小、备份数量及保留天数必须大于0")
	}

	return nil
}
