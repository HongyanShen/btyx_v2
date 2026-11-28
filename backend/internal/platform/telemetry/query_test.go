package telemetry

import (
	"strings"
	"testing"
)

func TestTraceSQL(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  string
	}{
		{`-- name: private
SELECT "users"."id" FROM "users" WHERE name=$1 AND status='private''value'`, `SELECT "users"."id" FROM "users" WHERE name=$1 AND status=?`},
		{`SELECT $tag$private$tag$, E'ab\'cd', 12.3e4, 99_000, true, NULL`, "SELECT ?, ?, ?, ?, ?, ?"},
		{"SELECT /* outer /* inner */ private */ $1", "SELECT $1"},
		{"SELECT 'private'\n'continued'", "SELECT ?"},
		{"UPDATE users SET name=$1 WHERE id=$2;", "UPDATE users SET name=$1 WHERE id=$2;"},
		{"SET application_name TO private", ""},
		{"SELECT $1; SET application_name TO private", ""},
		{"SELECT 'unterminated", ""},
		{"-- private", ""},
		{"SELECT " + strings.Repeat("x", 70000), ""},
	} {
		if got := traceSQL(tc.query); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
