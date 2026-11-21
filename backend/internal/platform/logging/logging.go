package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
)

type contextKey struct{}

type scope struct {
	base      zerolog.Logger
	logger    zerolog.Logger
	requestID string
}

var fallback = newScope(New(os.Stdout, "backend"), ksuid.New().String())

func New(writer io.Writer, process string) zerolog.Logger {
	return zerolog.New(zerolog.SyncWriter(writer)).
		With().
		Timestamp().
		Str("process", process).
		Logger()
}

func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, newScope(logger, ksuid.New().String()))
}

func NewRequest(ctx context.Context) context.Context {
	return WithRequestID(ctx, ksuid.New().String())
}

// WithRequestID restores correlation from a trusted persisted job, never raw HTTP input.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if _, err := ksuid.Parse(requestID); err != nil {
		requestID = ksuid.New().String()
	}

	return context.WithValue(ctx, contextKey{}, newScope(current(ctx).base, requestID))
}

func RequestID(ctx context.Context) string {
	return current(ctx).requestID
}

func From(ctx context.Context) *zerolog.Logger {
	return &current(ctx).logger
}

// Error records safe diagnostic categories, excluding arbitrary provider/error payloads.
func Error(ctx context.Context, err error) *zerolog.Event {
	event := From(ctx).Error().Str("error_type", fmt.Sprintf("%T", err))
	var databaseError interface {
		SQLState() string
	}
	if errors.As(err, &databaseError) {
		event.Str("sql_state", databaseError.SQLState())
	}

	if errors.Is(err, context.Canceled) {
		event.Str("error_code", "context_canceled")
	} else if errors.Is(err, context.DeadlineExceeded) {
		event.Str("error_code", "deadline_exceeded")
	}

	return event
}

func current(ctx context.Context) *scope {
	if attached, ok := ctx.Value(contextKey{}).(*scope); ok {
		return attached
	}

	return fallback
}

func newScope(base zerolog.Logger, requestID string) *scope {
	return &scope{
		base:      base,
		logger:    base.With().Str("request_id", requestID).Logger(),
		requestID: requestID,
	}
}
