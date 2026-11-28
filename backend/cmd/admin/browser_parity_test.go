package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captureAdminParity(t *testing.T, ctx context.Context, root, migrated, schema, username, password string) {
	t.Helper()
	if !strings.HasPrefix(schema, "btyx_test_") {
		t.Fatal("只能使用本轮的测试 schema 对照后台")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	baseline := exec.CommandContext(ctx, os.Getenv("BTYX_ADMIN_BASELINE_EXE"))
	baseline.Dir = filepath.Join(root, "backend")
	baseline.Env = append(os.Environ(),
		"BTYX_CONFIG="+filepath.Join(root, ".env"),
		"BTYX_ADMIN_DIST="+os.Getenv("BTYX_ADMIN_BASELINE_DIST"),
		"ADMIN_WEB_ADDR="+address,
		"ADMIN_WEB_SECURE_COOKIE=false",
		"PGSCHEMA="+schema,
		"LOG_DIR="+t.TempDir(),
	)
	baseline.Stdout = io.Discard
	baseline.Stderr = io.Discard
	if err := baseline.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = baseline.Process.Kill()
		_ = baseline.Wait()
	}()

	oldURL := "http://" + address
	client := &http.Client{Timeout: time.Second}
	ready := false
	for attempt := 0; attempt < 30; attempt++ {
		response, err := client.Get(oldURL + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		t.Fatal("旧版对照服务未就绪")
	}

	command := exec.CommandContext(ctx, "node", "browser/verify-admin-parity.mjs")
	command.Dir = filepath.Join(root, "backend", "cmd", "admin", "tests")
	command.Env = append(os.Environ(),
		"BTYX_BROWSER_BASELINE="+oldURL,
		"BTYX_BROWSER_URL="+migrated,
		"BTYX_BROWSER_USER="+username,
		"BTYX_BROWSER_PASSWORD="+password,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Edge 新旧版对照: %v\n%s", err, output)
	}

	t.Logf("%s", output)
}
