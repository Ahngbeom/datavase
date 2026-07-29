package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleColumns() []string { return []string{"id", "email", "note"} }

func sampleRows() [][]any {
	return [][]any{
		{int64(1), []byte("a@example.com"), nil},
		{int64(2), []byte("b@example.com"), []byte("hello")},
	}
}

func TestCSVWritesAHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, sampleColumns(), sampleRows()); err != nil {
		t.Fatalf("CSV() error = %v, want nil", err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("the output is not valid CSV: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want a header and two rows", len(records))
	}
	if records[0][0] != "id" || records[0][2] != "note" {
		t.Errorf("header = %v, want the column names", records[0])
	}
	if records[1][1] != "a@example.com" {
		t.Errorf("row = %v, want the email value", records[1])
	}
}

// NULL must be an empty field, not the four letters — otherwise a column
// containing the literal string "NULL" becomes indistinguishable from it.
func TestCSVWritesNullAsAnEmptyField(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]any{{nil, []byte("NULL")}}
	if err := CSV(&buf, []string{"a", "b"}, rows); err != nil {
		t.Fatalf("CSV() error = %v", err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("the output is not valid CSV: %v", err)
	}
	if records[1][0] != "" {
		t.Errorf("NULL was written as %q, want an empty field", records[1][0])
	}
	if records[1][1] != "NULL" {
		t.Errorf("the literal string was written as %q, want %q", records[1][1], "NULL")
	}
}

// Export keeps the full value; only the on-screen grid truncates.
func TestCSVDoesNotTruncate(t *testing.T) {
	long := strings.Repeat("x", 5000)

	var buf bytes.Buffer
	if err := CSV(&buf, []string{"v"}, [][]any{{[]byte(long)}}); err != nil {
		t.Fatalf("CSV() error = %v", err)
	}

	records, _ := csv.NewReader(&buf).ReadAll()
	if len(records[1][0]) != len(long) {
		t.Errorf("value was written with %d characters, want %d", len(records[1][0]), len(long))
	}
}

// Newlines and quotes inside a value must survive a round trip.
func TestCSVQuotesAwkwardValues(t *testing.T) {
	rows := [][]any{{[]byte("line1\nline2"), []byte(`say "hi"`), []byte("a,b")}}

	var buf bytes.Buffer
	if err := CSV(&buf, []string{"a", "b", "c"}, rows); err != nil {
		t.Fatalf("CSV() error = %v", err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("the output is not valid CSV: %v", err)
	}
	want := []string{"line1\nline2", `say "hi"`, "a,b"}
	for i, w := range want {
		if records[1][i] != w {
			t.Errorf("field %d = %q, want %q", i, records[1][i], w)
		}
	}
}

// A value beginning with =, +, - or @ is executed as a formula when the file
// is opened in a spreadsheet. Exporting query results into a spreadsheet is
// an everyday workflow, so this is a real path from a database row to code
// running on someone's machine.
func TestCSVNeutralisesFormulaInjection(t *testing.T) {
	dangerous := []string{
		`=cmd|' /c calc'!A1`,
		`+1+1`,
		`-1+1`,
		`@SUM(1:2)`,
		"\tSUM(1)",
		"\rSUM(1)",
	}

	for _, value := range dangerous {
		t.Run(value, func(t *testing.T) {
			var buf bytes.Buffer
			if err := CSV(&buf, []string{"v"}, [][]any{{[]byte(value)}}); err != nil {
				t.Fatalf("CSV() error = %v", err)
			}

			records, err := csv.NewReader(&buf).ReadAll()
			if err != nil {
				t.Fatalf("the output is not valid CSV: %v", err)
			}

			written := records[1][0]
			if written == value {
				t.Errorf("value %q was written unchanged; a spreadsheet would execute it", value)
			}
			// The original text must still be recoverable by a human.
			if !strings.Contains(written, strings.TrimLeft(value, "=+-@\t\r")) {
				t.Errorf("value was written as %q, losing the original content", written)
			}
		})
	}
}

// Ordinary values must not be mangled by the injection guard.
func TestCSVLeavesOrdinaryValuesAlone(t *testing.T) {
	ordinary := []string{"hello", "2026-07-28", "42", "a-b", "user@example.com", "한글"}

	for _, value := range ordinary {
		t.Run(value, func(t *testing.T) {
			var buf bytes.Buffer
			if err := CSV(&buf, []string{"v"}, [][]any{{[]byte(value)}}); err != nil {
				t.Fatalf("CSV() error = %v", err)
			}

			records, _ := csv.NewReader(&buf).ReadAll()
			if records[1][0] != value {
				t.Errorf("value %q was written as %q, want it unchanged", value, records[1][0])
			}
		})
	}
}

func TestCSVWithNoRowsStillWritesTheHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, sampleColumns(), nil); err != nil {
		t.Fatalf("CSV() error = %v", err)
	}

	records, _ := csv.NewReader(&buf).ReadAll()
	if len(records) != 1 {
		t.Fatalf("got %d records, want just the header", len(records))
	}
}

func TestJSONWritesAnArrayOfObjects(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleColumns(), sampleRows()); err != nil {
		t.Fatalf("JSON() error = %v, want nil", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("the output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("got %d objects, want 2", len(got))
	}
	if got[0]["email"] != "a@example.com" {
		t.Errorf("email = %v, want %q", got[0]["email"], "a@example.com")
	}
}

// JSON has a null of its own; using the string "NULL" would lose the
// distinction that JSON is able to express.
func TestJSONWritesNullAsNull(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, []string{"a"}, [][]any{{nil}}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("the output is not valid JSON: %v", err)
	}
	if value, present := got[0]["a"]; !present || value != nil {
		t.Errorf("a = %v (present %v), want an explicit null", value, present)
	}
}

// Numbers keep their type rather than becoming strings, which is what makes
// the output useful to jq and friends.
func TestJSONPreservesNumbers(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]any{{int64(42), 1.5, true}}
	if err := JSON(&buf, []string{"i", "f", "b"}, rows); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	if got := buf.String(); !strings.Contains(got, `"i": 42`) {
		t.Errorf("output = %s, want the integer unquoted", got)
	}
}

func TestJSONFormatsTime(t *testing.T) {
	ts := time.Date(2026, 7, 28, 15, 4, 5, 0, time.UTC)

	var buf bytes.Buffer
	if err := JSON(&buf, []string{"t"}, [][]any{{ts}}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got []map[string]any
	json.Unmarshal(buf.Bytes(), &got)
	if got[0]["t"] != "2026-07-28T15:04:05Z" {
		t.Errorf("time = %v, want RFC 3339", got[0]["t"])
	}
}

// Binary cannot go into JSON as text; base64 keeps it recoverable.
func TestJSONEncodesBinaryAsBase64(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, []string{"b"}, [][]any{{[]byte{0x00, 0xff}}}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("the output is not valid JSON: %v", err)
	}
	if got[0]["b"] == "" {
		t.Error("binary value was dropped")
	}
}

func TestJSONWithNoRowsWritesAnEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleColumns(), nil); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("output = %q, want an empty array", got)
	}
}

// Duplicate column names are legal in SQL ("SELECT id FROM a JOIN b") but not
// in a JSON object; the export must not silently drop one.
func TestJSONDisambiguatesDuplicateColumns(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]any{{int64(1), int64(2)}}
	if err := JSON(&buf, []string{"id", "id"}, rows); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("the output is not valid JSON: %v", err)
	}
	if len(got[0]) != 2 {
		t.Errorf("object = %v, want both columns preserved", got[0])
	}
}
