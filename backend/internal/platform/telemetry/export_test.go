package telemetry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"go.opentelemetry.io/otel"
)

func TestShutdownFlushesLocalAndOTLPExports(t *testing.T) {
	var received atomic.Bool
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.URL.Path != "/v1/traces" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-protobuf" || !bytes.Contains(body, []byte("export-test-span")) {
			t.Error("unexpected OTLP export")
		}

		received.Store(true)
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer collector.Close()
	directory := t.TempDir()
	previous := otel.GetTracerProvider()
	defer otel.SetTracerProvider(previous)
	closeTrace, err := Open(context.Background(), config.Config{
		TraceEndpoint:    collector.URL + "/v1/traces",
		TraceSampleRatio: 1,
		Logging:          config.Logging{Directory: directory},
	}, "export-test")
	if err != nil {
		t.Fatal(err)
	}

	_, span := otel.Tracer("test").Start(context.Background(), "export-test-span")
	span.End()
	closeTrace()
	local, err := os.ReadFile(filepath.Join(directory, "export-test.trace.jsonl"))
	if err != nil || !bytes.Contains(local, []byte(span.SpanContext().TraceID().String())) || !received.Load() {
		t.Fatalf("missing local or remote export: %v", err)
	}
}
