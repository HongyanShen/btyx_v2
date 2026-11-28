package telemetry

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"unicode"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func OpenPostgres(dsn string) (*sql.DB, error) {
	return otelsql.Open("postgres", dsn,
		otelsql.WithAttributes(attribute.String("db.system.name", "postgresql")),
		otelsql.WithAttributesGetter(func(_ context.Context, method otelsql.Method, query string, _ []driver.NamedValue) []attribute.KeyValue {
			switch method {
			case otelsql.MethodConnQuery, otelsql.MethodConnExec, otelsql.MethodConnPrepare, otelsql.MethodStmtQuery, otelsql.MethodStmtExec:
				if text := traceSQL(query); text != "" {
					return []attribute.KeyValue{attribute.String("db.query.text", text)}
				}
			}

			return nil
		}),
		otelsql.WithSpanNameFormatter(func(_ context.Context, method otelsql.Method, query string) string {
			query = strings.TrimSpace(query)
			if end := strings.IndexFunc(query, unicode.IsSpace); end >= 0 {
				query = query[:end]
			}

			switch op := strings.ToUpper(query); op {
			case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH":
				return "postgresql " + op
			}

			return string(method)
		}),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			Ping:                 true,
			OmitConnResetSession: true,
			RowsChildOfQuery:     true,
			DisableQuery:         true,
			RecordError:          func(error) bool { return false },
			SpanFilter: func(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) bool {
				return trace.SpanFromContext(ctx).IsRecording()
			},
		}),
		otelsql.WithSpanErrorAttributesGetter(func(err error) []attribute.KeyValue {
			if errors.Is(err, driver.ErrSkip) {
				return nil
			}

			return []attribute.KeyValue{attribute.String("error.type", "database_error")}
		}),
	)
}
