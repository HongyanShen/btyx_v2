package telemetry

import (
	"context"
	"io"
	"net/http"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Transport names only known operations. Signed paths, credentials and headers never become attributes.
type Transport string

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	name := string(t)
	switch req.URL.Path {
	case "/sns/jscode2session":
		name = "wechat.login"
	case "/cgi-bin/stable_token":
		name = "wechat.token"
	case "/cgi-bin/message/subscribe/send":
		name = "wechat.subscribe.send"
	case "/wxa/business/getuserphonenumber":
		name = "wechat.phone"
	}

	ctx, span := StartHTTP(req.Context(), name, req.Method)
	resp, err := http.DefaultTransport.RoundTrip(req.WithContext(ctx))
	if resp == nil {
		Finish(span, err)
	} else {
		resp.Body = ResponseBody(span, resp.StatusCode, resp.Body, err)
	}

	return resp, err
}

func StartHTTP(ctx context.Context, operation, method string) (context.Context, trace.Span) {
	return otel.Tracer("yanxuan/http").Start(ctx, operation, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attribute.String("http.request.method", method)))
}

func ResponseBody(span trace.Span, status int, body io.ReadCloser, err error) io.ReadCloser {
	span.SetAttributes(attribute.Int("http.response.status_code", status))
	if status >= 400 || err != nil {
		span.SetStatus(codes.Error, "")
	}

	if err != nil || body == nil || body == http.NoBody {
		span.End()
		return body
	}

	return &tracedBody{
		ReadCloser: body,
		span:       span,
	}
}

type tracedBody struct {
	io.ReadCloser
	span trace.Span
	once sync.Once
}

func (b *tracedBody) finish(err error) {
	b.once.Do(func() {
		if err != nil && err != io.EOF {
			b.span.SetStatus(codes.Error, "")
		}

		b.span.End()
	})
}

func (b *tracedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.finish(err)
	}

	return n, err
}

func (b *tracedBody) Close() error {
	err := b.ReadCloser.Close()
	b.finish(err)
	return err
}
