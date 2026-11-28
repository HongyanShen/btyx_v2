package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPAndJobsShareTraceWithoutExportingSecrets(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdk.NewTracerProvider(sdk.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(previous)
	ctx, parent := otel.Tracer("test").Start(context.Background(), "request")
	jobCtx, job := StartJob(context.Background(), Parent(ctx), "wechat_subscription", 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("traceparent") != "" {
			t.Error("propagated internal context to external provider")
		}

		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	req, err := http.NewRequestWithContext(jobCtx, http.MethodGet, server.URL+"/private-id?access_token=test-secret", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := (&http.Client{Transport: Transport("wechat")}).Do(req)
	if err != nil {
		t.Fatal(err)
	}

	resp.Body.Close()
	job.End()
	parent.End()
	spans := recorder.Ended()
	if len(spans) != 3 || spans[0].Parent().SpanID() != job.SpanContext().SpanID() || spans[1].Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatal("missing HTTP/queue span parent")
	}

	for _, span := range spans {
		for _, attr := range span.Attributes() {
			if strings.Contains(attr.Value.Emit(), "private-id") || strings.Contains(attr.Value.Emit(), "test-secret") {
				t.Fatal("sensitive URL exported")
			}
		}
	}
}
