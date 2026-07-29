package result

import (
	"strings"
	"testing"
	"time"
)

// NULL and the empty string must never render the same way: confusing them
// is how people end up wondering why "IS NULL" does not match.
func TestFormatDistinguishesNullFromEmpty(t *testing.T) {
	if got := Format(nil); got != NullText {
		t.Errorf("Format(nil) = %q, want %q", got, NullText)
	}
	if got := Format([]byte("")); got == NullText {
		t.Errorf("Format(empty string) = %q, which is indistinguishable from NULL", got)
	}
	if got := Format([]byte("")); got != "" {
		t.Errorf("Format(empty string) = %q, want %q", got, "")
	}
}

func TestFormatScalars(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "text bytes", in: []byte("bahn"), want: "bahn"},
		{name: "string", in: "bahn", want: "bahn"},
		{name: "int64", in: int64(42), want: "42"},
		{name: "negative int64", in: int64(-7), want: "-7"},
		{name: "float", in: 1.5, want: "1.5"},
		{name: "float without fraction", in: 2.0, want: "2"},
		{name: "true", in: true, want: "1"},
		{name: "false", in: false, want: "0"},
		{name: "korean text", in: []byte("한글"), want: "한글"},
		{name: "decimal as bytes", in: []byte("12345.67"), want: "12345.67"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.in); got != tt.want {
				t.Errorf("Format(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2026, 7, 28, 15, 4, 5, 0, time.UTC)

	if got := Format(ts); got != "2026-07-28 15:04:05" {
		t.Errorf("Format(time) = %q, want %q", got, "2026-07-28 15:04:05")
	}
}

// Raw binary would scramble the terminal, so it is shown as hex instead.
func TestFormatBinaryAsHex(t *testing.T) {
	got := Format([]byte{0x00, 0x01, 0xff})

	if strings.ContainsAny(got, "\x00\x01") {
		t.Fatalf("Format(binary) = %q, which contains raw control bytes", got)
	}
	if !strings.HasPrefix(got, "0x") {
		t.Errorf("Format(binary) = %q, want a 0x-prefixed hex rendering", got)
	}
}

// Control characters inside otherwise-valid text would break the grid layout.
func TestFormatEscapesControlCharactersInText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "newline", in: "a\nb", want: `a\nb`},
		{name: "tab", in: "a\tb", want: `a\tb`},
		{name: "carriage return", in: "a\rb", want: `a\rb`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format([]byte(tt.in)); got != tt.want {
				t.Errorf("Format(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// tview reads "[" as the start of a colour tag, so a value containing one
// would silently disappear or recolour the grid.
func TestEscapeTviewTagsLeavesTextVisible(t *testing.T) {
	got := EscapeTags("a[red]b")

	if got == "a[red]b" {
		t.Errorf("EscapeTags(%q) = %q, want the bracket escaped", "a[red]b", got)
	}
	if !strings.Contains(got, "red") {
		t.Errorf("EscapeTags(%q) = %q, want the text preserved", "a[red]b", got)
	}
}

func TestTruncateKeepsCellsNarrow(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{name: "shorter than the limit", in: "abc", limit: 5, want: "abc"},
		{name: "exactly the limit", in: "abcde", limit: 5, want: "abcde"},
		{name: "longer than the limit", in: "abcdefgh", limit: 5, want: "abcd…"},
		{name: "counts runes not bytes", in: "한글테스트입니다", limit: 4, want: "한글테…"},
		{name: "no limit", in: "abcdefgh", limit: 0, want: "abcdefgh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.in, tt.limit); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}
