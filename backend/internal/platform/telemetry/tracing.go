package telemetry

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Open(ctx context.Context, cfg config.Config, service string) (func(), error) {
	settings := cfg.Logging
	file := &lumberjack.Logger{
		Filename:   filepath.Join(settings.Directory, service+".trace.jsonl"),
		MaxSize:    settings.MaxSizeMB,
		MaxAge:     settings.MaxAgeDays,
		MaxBackups: settings.MaxBackups,
		Compress:   settings.Compress,
	}
	local, err := stdouttrace.New(stdouttrace.WithWriter(file))
	if err != nil {
		return nil, err
	}

	opts := []sdk.TracerProviderOption{
		sdk.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
		sdk.WithSampler(sdk.ParentBased(sdk.TraceIDRatioBased(cfg.TraceSampleRatio))),
		sdk.WithBatcher(local),
	}
	if cfg.TraceEndpoint != "" {
		endpoint, err := url.Parse(cfg.TraceEndpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return nil, errors.New("TRACE_OTLP_ENDPOINT 需为 OTLP HTTP traces 地址")
		}

		remote, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.TraceEndpoint))
		if err != nil {
			return nil, err
		}

		opts = append(opts, sdk.WithBatcher(remote))
	}

	// Initialize the SQL scanner before the first request can hold a database lock.
	if cfg.TraceSampleRatio > 0 {
		traceSQL("SELECT 1")
	}

	provider := sdk.NewTracerProvider(opts...)
	otel.SetTracerProvider(provider)
	return func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Shutdown(shutdown)
		_ = file.Close()
	}, nil
}

// SetUser identifies the account associated with the current operation.
func SetUser(ctx context.Context, id string, role enum.Role) {
	if id == "" {
		return
	}

	trace.SpanFromContext(ctx).SetAttributes(semconv.UserID(id), semconv.UserRoles(string(role)))
}

func Parent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

func StartJob(ctx context.Context, parent, kind string, attempt int) (context.Context, trace.Span) {
	ctx = propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": parent})
	return otel.Tracer("yanxuan/worker").Start(ctx, kind,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.Int("job.attempt", attempt)),
	)
}

func Finish(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, "")
	}

	span.End()
}
