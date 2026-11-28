package logging

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/config"
)

func TestFileLoggingAutomaticallyRotatesCompressesAndCleansOldFiles(t *testing.T) {
	directory := t.TempDir()
	expired := filepath.Join(directory, "testapi-2000-01-01T00-00-00.000.log")
	if err := os.WriteFile(expired, []byte("old log"), 0600); err != nil {
		t.Fatal(err)
	}

	unrelated := filepath.Join(directory, "another-process.log")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	logger, closer, err := Open(config.Logging{
		Directory:  directory,
		MaxSizeMB:  1,
		MaxBackups: 1,
		MaxAgeDays: 1,
		Compress:   true,
	}, "testapi")
	if err != nil {
		t.Fatal(err)
	}

	ctx := WithLogger(context.Background(), logger)
	for index := 0; index < 5; index++ {
		Info(ctx).Str("payload", strings.Repeat("x", 300*1024)).Msg("rotation test")
	}

	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	var archives []string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		archives, err = filepath.Glob(filepath.Join(directory, "testapi-*.log.gz"))
		if err != nil {
			t.Fatal(err)
		}

		if len(archives) == 1 {
			if _, err := os.Stat(expired); os.IsNotExist(err) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	if len(archives) != 1 {
		t.Fatalf("size rotation did not produce compressed archive: %v", archives)
	}

	file, err := os.Open(archives[0])
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}

	defer compressed.Close()
	archived, err := io.ReadAll(compressed)
	if err != nil {
		t.Fatal(err)
	}

	current, err := os.ReadFile(filepath.Join(directory, "testapi.log"))
	if err != nil || len(current) > 1024*1024 {
		t.Fatalf("current log was lost or exceeded size limit: bytes=%d err=%v", len(current), err)
	}

	count := 0
	for _, content := range [][]byte{archived, current} {
		for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
			var entry struct {
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.RequestID != RequestID(ctx) {
				t.Fatal("rotation lost a structured log or its request ID")
			}

			count++
		}
	}

	if count != 5 {
		t.Fatalf("rotation lost events: %d", count)
	}

	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Fatal("expired backup was not cleaned")
	}

	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("rotation removed another process's log")
	}
}

func TestFileLoggerRejectsUnwritableDestinationsAndInvalidRetention(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "testapi.log"), 0700); err != nil {
		t.Fatal(err)
	}

	settings := config.Logging{
		Directory:  directory,
		MaxSizeMB:  1,
		MaxBackups: 1,
		MaxAgeDays: 1,
	}
	if _, closer, err := Open(settings, "testapi"); err == nil {
		_ = closer.Close()
		t.Fatal("unwritable log destination reported success")
	}

	settings.MaxBackups = 0
	if _, closer, err := Open(settings, "invalid"); err == nil {
		_ = closer.Close()
		t.Fatal("unbounded log retention was accepted")
	}
}
