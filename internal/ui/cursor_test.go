package ui

import "testing"

// The editor reports the caret as a row and column; the parser needs a byte
// offset. Getting this wrong runs a different statement than the one under
// the cursor, which against production is exactly the mistake this tool
// exists to prevent.
func TestOffsetAt(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		row    int
		column int
		want   int
	}{
		{name: "start of the first line", text: "SELECT 1", row: 0, column: 0, want: 0},
		{name: "middle of the first line", text: "SELECT 1", row: 0, column: 3, want: 3},
		{name: "end of the first line", text: "SELECT 1", row: 0, column: 8, want: 8},
		{name: "start of the second line", text: "SELECT 1;\nSELECT 2", row: 1, column: 0, want: 10},
		{name: "middle of the second line", text: "SELECT 1;\nSELECT 2", row: 1, column: 6, want: 16},
		{name: "third line", text: "a\nbb\nccc", row: 2, column: 2, want: 7},
		{name: "empty text", text: "", row: 0, column: 0, want: 0},
		{name: "empty line between statements", text: "a\n\nb", row: 2, column: 0, want: 3},

		// A column is a rune index, but the parser works in bytes.
		{name: "after multibyte characters", text: "SELECT '한글'", row: 0, column: 9, want: 11},
		{name: "korean on the second line", text: "a\n한글", row: 1, column: 1, want: 5},

		// Out-of-range positions are clamped rather than panicking; the
		// editor can report a caret past the end while text is changing.
		{name: "row past the end", text: "a\nb", row: 9, column: 0, want: 3},
		{name: "column past the end of a line", text: "ab\ncd", row: 0, column: 99, want: 2},
		{name: "negative row", text: "ab", row: -1, column: 0, want: 0},
		{name: "negative column", text: "ab", row: 0, column: -1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := offsetAt(tt.text, tt.row, tt.column); got != tt.want {
				t.Errorf("offsetAt(%q, %d, %d) = %d, want %d", tt.text, tt.row, tt.column, got, tt.want)
			}
		})
	}
}

// Whatever the caret position, the offset must be a valid index into the
// text; an invalid one would panic when the parser slices the buffer.
func TestOffsetAtAlwaysProducesAValidIndex(t *testing.T) {
	texts := []string{"", "a", "SELECT 1;\nSELECT 2;\n", "한글\n테스트", "\n\n\n"}

	for _, text := range texts {
		for row := -2; row < 6; row++ {
			for column := -2; column < 12; column++ {
				got := offsetAt(text, row, column)
				if got < 0 || got > len(text) {
					t.Fatalf("offsetAt(%q, %d, %d) = %d, outside [0,%d]",
						text, row, column, got, len(text))
				}
				_ = text[:got] // must not panic
			}
		}
	}
}
