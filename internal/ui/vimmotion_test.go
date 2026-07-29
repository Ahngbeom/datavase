package ui

import "testing"

// vim's word motions are not the ones the DataGrip keyboard uses. "w" lands
// on the start of the next word where Ctrl+→ lands on the end of this one,
// and the difference is exactly what makes "dw" take the trailing space.
func TestWordStartRight(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "from a word to the start of the next",
			from: "SELECT| id",
			want: "SELECT |id",
		},
		{
			name: "from inside a word",
			from: "SEL|ECT id",
			want: "SELECT |id",
		},
		{
			name: "over punctuation",
			from: "|a.b",
			want: "a.|b",
		},
		{
			name: "stops at the end of the line rather than joining the next",
			from: "SELECT |id\nFROM t",
			want: "SELECT id|\nFROM t",
		},
		{
			name: "at the end of the buffer",
			from: "SELECT id|",
			want: "SELECT id|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, from := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := wordStartRight(text, from); got != want {
				t.Errorf("wordStartRight(%q, %d) = %d, want %d", text, from, got, want)
			}
		})
	}
}

func TestFirstNonBlankAt(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "past the indentation",
			from: "  SELECT|",
			want: "  |SELECT",
		},
		{
			name: "already there",
			from: "SEL|ECT",
			want: "|SELECT",
		},
		{
			name: "a blank line stays put",
			from: "a\n  |\nb",
			want: "a\n  |\nb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, from := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := firstNonBlankAt(text, from); got != want {
				t.Errorf("firstNonBlankAt(%q, %d) = %d, want %d", text, from, got, want)
			}
		})
	}
}

// h and l stay on their line; in vim they do not wrap, and a cursor that
// silently jumped to the previous line would make "dl" delete a newline.
func TestCharMotions(t *testing.T) {
	tests := []struct {
		name   string
		motion func(string, int) int
		from   string
		want   string
	}{
		{name: "left", motion: charLeft, from: "abc|", want: "ab|c"},
		{name: "left stops at the line start", motion: charLeft, from: "ab\n|cd", want: "ab\n|cd"},
		{name: "left over a multi-byte rune", motion: charLeft, from: "a한|", want: "a|한"},
		{name: "right", motion: charRight, from: "|abc", want: "a|bc"},
		{name: "right stops at the line end", motion: charRight, from: "ab|\ncd", want: "ab|\ncd"},
		{name: "right over a multi-byte rune", motion: charRight, from: "|한a", want: "한|a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, from := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := tt.motion(text, from); got != want {
				t.Errorf("%s(%q, %d) = %d, want %d", tt.name, text, from, got, want)
			}
		})
	}
}

// j and k keep the column, which is what makes them usable for moving down a
// list of columns in a SELECT.
func TestLineMotions(t *testing.T) {
	tests := []struct {
		name   string
		motion func(string, int) int
		from   string
		want   string
	}{
		{name: "down", motion: lineDown, from: "ab|cd\nefgh", want: "abcd\nef|gh"},
		{name: "down onto a shorter line", motion: lineDown, from: "abcd|\nef", want: "abcd\nef|"},
		{name: "down from the last line stays", motion: lineDown, from: "ab\ncd|", want: "ab\ncd|"},
		{name: "up", motion: lineUp, from: "abcd\nef|gh", want: "ab|cd\nefgh"},
		{name: "up from the first line stays", motion: lineUp, from: "a|bcd", want: "a|bcd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, from := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := tt.motion(text, from); got != want {
				t.Errorf("%s(%q, %d) = %d, want %d", tt.name, text, from, got, want)
			}
		})
	}
}
