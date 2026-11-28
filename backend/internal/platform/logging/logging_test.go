package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/otel/trace"
)

func TestLevelHelpersKeepSpanAndStructuredFields(t *testing.T) {
	var output bytes.Buffer
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{2},
	})
	ctx := trace.ContextWithSpanContext(WithLogger(context.Background(), New(&output, "worker")), span)
	Info(ctx).Int("attempt", 1).Msg("started")
	Warn(ctx).Int("attempt", 2).Msg("retry")
	Error(ctx, nil).Int("attempt", 3).Msg("failed")

	decoder := json.NewDecoder(&output)
	for i, level := range []string{"info", "warn", "error"} {
		var entry struct {
			Level     string  `json:"level"`
			Attempt   int     `json:"attempt"`
			RequestID string  `json:"request_id"`
			TraceID   string  `json:"trace_id"`
			SpanID    string  `json:"span_id"`
			ErrorType *string `json:"error_type"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}

		if entry.Level != level || entry.Attempt != i+1 || entry.RequestID != RequestID(ctx) || entry.TraceID != span.TraceID().String() || entry.SpanID != span.SpanID().String() || entry.ErrorType != nil {
			t.Fatalf("incorrect %s event: %+v", level, entry)
		}
	}
}

func TestRequestContextKeepsCorrelationAcrossDerivedContexts(t *testing.T) {
	var output bytes.Buffer
	root := WithLogger(context.Background(), New(&output, "test"))
	request := NewRequest(root)
	child, cancel := context.WithCancel(request)
	defer cancel()

	Info(request).Msg("business action")
	Warn(child).Msg("nested business action")
	other := NewRequest(root)
	Info(other).Msg("other request")
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 || RequestID(request) == RequestID(other) {
		t.Fatalf("request isolation failed: %s", output.String())
	}

	for index, line := range lines {
		var entry struct {
			RequestID string `json:"request_id"`
			Process   string `json:"process"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}

		if _, err := ksuid.Parse(entry.RequestID); err != nil || entry.Process != "test" || strings.Count(line, `"request_id"`) != 1 {
			t.Fatalf("invalid contextual log: %s", line)
		}

		if index < 2 && entry.RequestID != RequestID(request) {
			t.Fatalf("derived context lost request ID: %s", line)
		}
	}
}

func TestRestoringRequestIDAndLoggingErrorsDoesNotExposePayloads(t *testing.T) {
	var output bytes.Buffer
	root := WithLogger(context.Background(), New(&output, "worker"))
	origin := RequestID(NewRequest(root))
	restored := WithRequestID(NewRequest(root), origin)
	Error(restored, errors.New("secret-token-and-password")).Msg("operation failed")
	if RequestID(restored) != origin || !strings.Contains(output.String(), origin) || strings.Contains(output.String(), "secret-token-and-password") {
		t.Fatalf("unsafe or uncorrelated error: %s", output.String())
	}

	invalid := WithRequestID(root, "untrusted\nrequest-id")
	if _, err := ksuid.Parse(RequestID(invalid)); err != nil {
		t.Fatal("invalid restored request ID was not replaced")
	}

	if _, err := ksuid.Parse(RequestID(context.Background())); err != nil {
		t.Fatal("background logging has no fallback correlation ID")
	}
}

func TestConcurrentRequestLogsKeepSeparateIDs(t *testing.T) {
	var output bytes.Buffer
	root := WithLogger(context.Background(), New(&output, "test"))
	var wait sync.WaitGroup
	for index := 0; index < 12; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ctx := NewRequest(root)
			Info(ctx).Msg("first action")
			Info(ctx).Msg("second action")
		}()
	}

	wait.Wait()
	counts := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var entry struct {
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}

		counts[entry.RequestID]++
	}

	if len(counts) != 12 {
		t.Fatalf("concurrent requests shared correlation IDs: %v", counts)
	}

	for requestID, count := range counts {
		if count != 2 || requestID == "" {
			t.Fatalf("request logs crossed contexts: %v", counts)
		}
	}
}
