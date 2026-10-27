package headless

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func decodeLines(t *testing.T, b *bytes.Buffer) []any {
	t.Helper()

	var out []any
	sc := bufio.NewScanner(b)
	for sc.Scan() {
		var v any
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			t.Fatalf("line %q is not JSON: %v", sc.Text(), err)
		}
		out = append(out, v)
	}
	return out
}

func TestTheResultIsAHeaderThenARowPerLineThenASummary(t *testing.T) {
	var b bytes.Buffer
	l := newLines(&b, 10)

	if err := l.header([]string{"id", "email"}, []string{"BIGINT", "VARCHAR"}); err != nil {
		t.Fatal(err)
	}
	if err := l.add([][]any{{int64(1), []byte("ada@example.com")}, {int64(2), nil}}); err != nil {
		t.Fatal(err)
	}
	if err := l.summary(1500 * time.Microsecond); err != nil {
		t.Fatal(err)
	}

	got := decodeLines(t, &b)
	if len(got) != 4 {
		t.Fatalf("wrote %d lines, want 4: %v", len(got), got)
	}

	header := got[0].(map[string]any)
	if cols := header["columns"].([]any); len(cols) != 2 || cols[1] != "email" {
		t.Errorf("header columns = %v", header["columns"])
	}
	if types := header["types"].([]any); types[0] != "BIGINT" {
		t.Errorf("header types = %v", header["types"])
	}

	// A row is an array, so a reader tells it from the two objects by its
	// first byte, and a column named "rows" can never pass for the summary.
	if row := got[1].([]any); row[0] != float64(1) || row[1] != "ada@example.com" {
		t.Errorf("first row = %v; bytes must arrive as text, numbers as numbers", row)
	}
	if row := got[2].([]any); row[1] != nil {
		t.Errorf("NULL arrived as %v, want null", row[1])
	}

	summary := got[3].(map[string]any)
	if summary["rows"] != float64(2) || summary["truncated"] != false || summary["elapsed_ms"] != float64(1) {
		t.Errorf("summary = %v, want 2 rows, not truncated, 1ms", summary)
	}
}

// The runner fetches one row more than the cap for exactly this: a result
// that happens to be the size of the cap was otherwise reported as cut short.
func TestOnlyARowBeyondTheCapMarksTheResultTruncated(t *testing.T) {
	for _, tc := range []struct {
		rows      int
		truncated bool
		written   int
	}{
		{rows: 2, truncated: false, written: 2},
		{rows: 3, truncated: true, written: 2},
	} {
		var b bytes.Buffer
		l := newLines(&b, 2)
		l.header([]string{"n"}, []string{"INT"})

		var batch [][]any
		for i := range tc.rows {
			batch = append(batch, []any{int64(i)})
		}
		// Split across two batches, as the stream delivers them.
		l.add(batch[:1])
		l.add(batch[1:])
		l.summary(0)

		got := decodeLines(t, &b)
		if rows := len(got) - 2; rows != tc.written {
			t.Errorf("%d rows fetched: wrote %d, want %d", tc.rows, rows, tc.written)
		}
		summary := got[len(got)-1].(map[string]any)
		if summary["truncated"] != tc.truncated || summary["rows"] != float64(tc.written) {
			t.Errorf("%d rows fetched: summary = %v, want truncated=%v rows=%d", tc.rows, summary, tc.truncated, tc.written)
		}
	}
}

// A JSON encoder escapes <, > and & by default, which is safe for HTML and
// unreadable in a terminal or a model's context; nothing here is HTML.
func TestTextIsWrittenAsItIs(t *testing.T) {
	var b bytes.Buffer
	l := newLines(&b, 1)
	l.header([]string{"s"}, []string{"TEXT"})
	l.add([][]any{{"<b>&</b>"}})

	if !strings.Contains(b.String(), `"<b>&</b>"`) {
		t.Errorf("output = %q, want the text unescaped", b.String())
	}
}
