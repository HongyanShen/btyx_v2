package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Open(settings config.Logging, process string) (zerolog.Logger, io.Closer, error) {
	if err := settings.Validate(); err != nil {
		return zerolog.Logger{}, nil, err
	}

	if !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(process) {
		return zerolog.Logger{}, nil, fmt.Errorf("日志进程名称无效")
	}

	if err := os.MkdirAll(settings.Directory, 0700); err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	filename := filepath.Join(settings.Directory, process+".log")
	existing, err := os.Lstat(filename)
	if err == nil && !existing.Mode().IsRegular() {
		return zerolog.Logger{}, nil, fmt.Errorf("日志目标必须是普通文件")
	}

	if err != nil && !os.IsNotExist(err) {
		return zerolog.Logger{}, nil, fmt.Errorf("检查日志目标失败: %w", err)
	}

	writer := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    settings.MaxSizeMB,
		MaxBackups: settings.MaxBackups,
		MaxAge:     settings.MaxAgeDays,
		Compress:   settings.Compress,
	}
	if _, err := writer.Write(nil); err != nil {
		_ = writer.Close()

		return zerolog.Logger{}, nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	return New(writer, process), writer, nil
}
