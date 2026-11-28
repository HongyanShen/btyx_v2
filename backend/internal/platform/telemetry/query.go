package telemetry

import (
	"strings"
	"unicode/utf8"

	pg "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
)

// traceSQL emits single-statement business queries. Only token text is copied,
// never the unclassified gaps, driver arguments, or a scanner error message.
func traceSQL(query string) string {
	if len(query) == 0 || len(query) > 64*1024 || !utf8.ValidString(query) || strings.IndexByte(query, 0) >= 0 {
		return ""
	}

	scan, err := pgquery.Scan(query)
	if err != nil {
		return ""
	}

	var sql strings.Builder
	var previousEnd int32
	var ended bool
	for _, token := range scan.Tokens {
		if token.Token == pg.Token_SQL_COMMENT || token.Token == pg.Token_C_COMMENT {
			continue
		}

		if ended || token.Start < previousEnd || token.End <= token.Start || int(token.End) > len(query) {
			return ""
		}

		if sql.Len() == 0 {
			switch token.Token {
			case pg.Token_SELECT, pg.Token_INSERT, pg.Token_UPDATE, pg.Token_DELETE_P, pg.Token_WITH, pg.Token_MERGE:
			default:
				return ""
			}
		} else if token.Start > previousEnd {
			sql.WriteByte(' ')
		}

		switch token.Token {
		case pg.Token_SCONST, pg.Token_USCONST, pg.Token_BCONST, pg.Token_XCONST,
			pg.Token_ICONST, pg.Token_FCONST, pg.Token_TRUE_P, pg.Token_FALSE_P, pg.Token_NULL_P:
			sql.WriteByte('?')
		default:
			sql.WriteString(query[token.Start:token.End])
		}

		previousEnd = token.End
		ended = token.Token == pg.Token_ASCII_59
	}

	return sql.String()
}
