package flow

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryTracer emits <self>→postgres for every statement, labelled with its
// verb and first table ("SELECT issues"). The SQL text and its arguments never
// leave the process. Install it on a pool's ConnConfig.Tracer; with no
// publisher installed it costs one atomic load per query.
type QueryTracer struct{}

type queryStartKey struct{}

type queryStart struct {
	at    time.Time
	label string
}

func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !Active() {
		return ctx
	}
	return context.WithValue(ctx, queryStartKey{}, queryStart{at: time.Now(), label: sqlLabel(data.SQL)})
}

func (QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(queryStartKey{}).(queryStart)
	if !ok {
		return
	}
	payload := ""
	if data.Err != nil {
		payload = "error"
	}
	Emit(ctx, Event{
		From: Self(), To: Postgres, Kind: KindSQL,
		Label: start.label, Payload: payload,
		Ms: float64(time.Since(start.at).Microseconds()) / 1000,
	})
}

// sqlLabel returns "VERB table" for a statement, "VERB" for one with no
// table (BEGIN, COMMIT), or "query" when it cannot tell. Only a keyword from
// a fixed list and an identifier the code itself wrote can come out of it.
func sqlLabel(sql string) string {
	words := sqlWords(sql, 64)
	if len(words) == 0 {
		return "query"
	}
	verb := words[0]
	var after string
	switch verb {
	case "select", "delete", "with":
		after = "from"
	case "insert":
		after = "into"
	case "update":
		after = "update"
	case "begin", "commit", "rollback", "savepoint", "release", "listen", "notify", "set", "show":
		return strings.ToUpper(verb)
	default:
		return "query"
	}
	// INSERT … ON CONFLICT is how every write here upserts; say so.
	if verb == "insert" && containsWords(words, "on", "conflict") {
		verb = "upsert"
	}
	// The first table after the keyword; FROM (subquery) moves on to the
	// subquery's own FROM.
	for i, w := range words {
		if w == after && i+1 < len(words) && isIdentifier(words[i+1]) {
			return strings.ToUpper(verb) + " " + words[i+1]
		}
	}
	return strings.ToUpper(verb)
}

// sqlWords splits the start of sql into lower-case words, skipping comments
// and string literals, up to limit words.
func sqlWords(sql string, limit int) []string {
	var words []string
	var cur strings.Builder
	end := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(sql) && len(words) < limit; i++ {
		c := sql[i]
		switch {
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			end()
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(sql) && sql[i+1] == '*':
			end()
			if j := strings.Index(sql[i+2:], "*/"); j >= 0 {
				i += j + 3
			} else {
				i = len(sql)
			}
		case c == '\'':
			end()
			if j := strings.IndexByte(sql[i+1:], '\''); j >= 0 {
				i += j + 1
			} else {
				i = len(sql)
			}
		case c >= 'A' && c <= 'Z':
			cur.WriteByte(c + 'a' - 'A')
		case c == '_' || c == '.' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			cur.WriteByte(c)
		default:
			end()
		}
	}
	end()
	return words
}

func containsWords(words []string, a, b string) bool {
	for i := 0; i+1 < len(words); i++ {
		if words[i] == a && words[i+1] == b {
			return true
		}
	}
	return false
}

func isIdentifier(s string) bool {
	if s == "" || len(s) > 63 || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	switch s {
	case "select", "lateral", "only", "unnest", "generate_series", "jsonb_to_recordset", "json_to_recordset":
		return false
	}
	return true
}
