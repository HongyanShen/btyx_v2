package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggingDefaultsResolveRelativeToConfigurationFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".env")
	content := "PGSCHEMA=btyx_test_config\nENCRYPTION_KEY=" + strings.Repeat("a", 64) + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if settings.Logging.Directory != filepath.Join(directory, "artifacts", "logs") || settings.Logging.MaxSizeMB != 20 || settings.Logging.MaxBackups != 7 || settings.Logging.MaxAgeDays != 14 || !settings.Logging.Compress {
		t.Fatalf("invalid logging defaults: %+v", settings.Logging)
	}

	t.Setenv("LOG_DIR", "custom-logs")
	t.Setenv("LOG_MAX_SIZE_MB", "5")
	t.Setenv("LOG_MAX_BACKUPS", "3")
	t.Setenv("LOG_MAX_AGE_DAYS", "2")
	t.Setenv("LOG_COMPRESS", "false")
	settings, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if settings.Logging.Directory != filepath.Join(directory, "custom-logs") || settings.Logging.MaxSizeMB != 5 || settings.Logging.MaxBackups != 3 || settings.Logging.MaxAgeDays != 2 || settings.Logging.Compress {
		t.Fatalf("logging overrides ignored: %+v", settings.Logging)
	}

	t.Setenv("LOG_MAX_BACKUPS", "0")
	if _, err := Load(path); err == nil {
		t.Fatal("invalid retention did not stop startup")
	}
}
