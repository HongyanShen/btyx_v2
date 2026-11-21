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
)

func TestRequestContextKeepsCorrelationAcrossDerivedContexts(t *testing.T) {
	var output bytes.Buffer
	root := WithLogger(context.Background(), New(&output, "test"))
	request := NewRequest(root)
	child, cancel := context.WithCancel(request)
	defer cancel()

	From(request).Info().Msg("business action")
	From(child).Warn().Msg("nested business action")
	other := NewRequest(root)
	From(other).Info().Msg("other request")
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
			From(ctx).Info().Msg("first action")
			From(ctx).Info().Msg("second action")
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
