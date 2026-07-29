package ui

import "testing"

// The tests mark the caret with "|" so each case reads as the keystroke it
// describes: the text before the move, and where the caret should land.
func caretCase(t *testing.T, marked string) (text string, offset int) {
	t.Helper()

	for i := 0; i < len(marked); i++ {
		if marked[i] == '|' {
			return marked[:i] + marked[i+1:], i
		}
	}
	t.Fatalf("test text %q has no caret marker", marked)
	return "", 0
}

func TestWordLeft(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "from the end of a word to its start",
			from: "SELECT id|",
			want: "SELECT |id",
		},
		{
			name: "from a word start to the previous word start",
			from: "SELECT |id",
			want: "|SELECT id",
		},
		{
			name: "skips the space between words",
			from: "SELECT  |id",
			want: "|SELECT  id",
		},
		{
			name: "from the middle of a word to its start",
			from: "SELECT us|ers",
			want: "SELECT |users",
		},
		// A SQL identifier is one word. tview's own movement splits on the
		// underscore, which is wrong for this editor.
		{
			name: "underscored identifier is a single word",
			from: "SELECT user_id|",
			want: "SELECT |user_id",
		},
		{
			name: "stops at punctuation",
			from: "u.id|",
			want: "u.|id",
		},
		{
			name: "skips punctuation to reach the previous word",
			from: "u.|id",
			want: "|u.id",
		},
		{
			name: "crosses a newline to the previous line's last word",
			from: "SELECT 1\n|FROM t",
			want: "SELECT |1\nFROM t",
		},
		{
			name: "already at the start stays put",
			from: "|SELECT",
			want: "|SELECT",
		},
		{
			name: "leading whitespace only",
			from: "   |",
			want: "|   ",
		},
		{
			name: "multibyte identifier",
			from: "SELECT 한글컬럼|",
			want: "SELECT |한글컬럼",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, offset := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := wordLeft(text, offset); got != want {
				t.Errorf("wordLeft(%q, %d) = %d, want %d", text, offset, got, want)
			}
		})
	}
}

func TestWordRight(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "from a word start to its end",
			from: "SELECT |id",
			want: "SELECT id|",
		},
		{
			name: "from a word end to the next word end",
			from: "SELECT| id",
			want: "SELECT id|",
		},
		{
			name: "from the middle of a word to its end",
			from: "SEL|ECT id",
			want: "SELECT| id",
		},
		{
			name: "underscored identifier is a single word",
			from: "|user_id, x",
			want: "user_id|, x",
		},
		{
			name: "skips punctuation to reach the next word",
			from: "u|.id",
			want: "u.id|",
		},
		{
			name: "crosses a newline to the next line's first word",
			from: "SELECT 1|\nFROM t",
			want: "SELECT 1\nFROM| t",
		},
		{
			name: "already at the end stays put",
			from: "SELECT|",
			want: "SELECT|",
		},
		{
			name: "trailing whitespace only",
			from: "|   ",
			want: "   |",
		},
		{
			name: "multibyte identifier",
			from: "SELECT |한글컬럼 x",
			want: "SELECT 한글컬럼| x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, offset := caretCase(t, tt.from)
			_, want := caretCase(t, tt.want)

			if got := wordRight(text, offset); got != want {
				t.Errorf("wordRight(%q, %d) = %d, want %d", text, offset, got, want)
			}
		})
	}
}

// Word movement is deliberately asymmetric — left seeks a word's start,
// right seeks its end — so a round trip does not return to where it began.
// Pinning it stops a later "fix" from breaking the familiar behaviour.
func TestWordMovementIsAsymmetric(t *testing.T) {
	const text = "SELECT users FROM t"

	// From the middle of "users".
	start := 9
	back := wordLeft(text, start)
	forward := wordRight(text, back)

	if text[back:forward] != "users" {
		t.Errorf("wordLeft then wordRight spans %q, want %q", text[back:forward], "users")
	}
}

func TestLineStartAndEnd(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		wantStart string
		wantEnd   string
	}{
		{
			name:      "single line",
			from:      "SELECT |1",
			wantStart: "|SELECT 1",
			wantEnd:   "SELECT 1|",
		},
		{
			name:      "middle line of three",
			from:      "a\nbb|b\nc",
			wantStart: "a\n|bbb\nc",
			wantEnd:   "a\nbbb|\nc",
		},
		{
			name:      "already at the start",
			from:      "a\n|bbb",
			wantStart: "a\n|bbb",
			wantEnd:   "a\nbbb|",
		},
		{
			name:      "empty line",
			from:      "a\n|\nb",
			wantStart: "a\n|\nb",
			wantEnd:   "a\n|\nb",
		},
		{
			name:      "indented line keeps the indentation before the caret",
			from:      "    SELECT| 1",
			wantStart: "|    SELECT 1",
			wantEnd:   "    SELECT 1|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, offset := caretCase(t, tt.from)
			_, wantStart := caretCase(t, tt.wantStart)
			_, wantEnd := caretCase(t, tt.wantEnd)

			if got := lineStartAt(text, offset); got != wantStart {
				t.Errorf("lineStartAt(%q, %d) = %d, want %d", text, offset, got, wantStart)
			}
			if got := lineEndAt(text, offset); got != wantEnd {
				t.Errorf("lineEndAt(%q, %d) = %d, want %d", text, offset, got, wantEnd)
			}
		})
	}
}

func TestDeleteWordLeft(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "deletes the word before the caret",
			from: "SELECT users|",
			want: "SELECT ",
		},
		{
			name: "deletes the whole identifier",
			from: "SELECT user_id|",
			want: "SELECT ",
		},
		{
			name: "from the middle of a word deletes only up to the caret",
			from: "SELECT us|ers",
			want: "SELECT ers",
		},
		{
			name: "deletes trailing space with the word",
			from: "SELECT users |",
			want: "SELECT ",
		},
		{
			name: "at the start does nothing",
			from: "|SELECT",
			want: "SELECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, offset := caretCase(t, tt.from)

			if got := applyEdit(text, deleteWordLeft(text, offset)); got != tt.want {
				t.Errorf("deleteWordLeft(%q, %d) = %q, want %q", text, offset, got, tt.want)
			}
		})
	}
}

func TestDeleteToLineStart(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "clears the line before the caret",
			from: "SELECT 1|",
			want: "",
		},
		{
			name: "keeps what follows the caret",
			from: "SELECT |1",
			want: "1",
		},
		{
			name: "keeps earlier lines",
			from: "a\nSELECT 1|\nb",
			want: "a\n\nb",
		},
		{
			name: "at the line start does nothing",
			from: "a\n|b",
			want: "a\nb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, offset := caretCase(t, tt.from)

			if got := applyEdit(text, deleteToLineStart(text, offset)); got != tt.want {
				t.Errorf("deleteToLineStart(%q, %d) = %q, want %q", text, offset, got, tt.want)
			}
		})
	}
}

// Movement runs on a keystroke and the caret can be reported outside the
// buffer while an edit is in flight; a panic there would lose the user's SQL.
func TestMotionToleratesAnyOffset(t *testing.T) {
	texts := []string{"", "a", "SELECT user_id FROM t", "한글\n테스트", "\n\n\n", "   "}

	for _, text := range texts {
		for offset := -3; offset <= len(text)+3; offset++ {
			for name, fn := range map[string]func(string, int) int{
				"wordLeft":    wordLeft,
				"wordRight":   wordRight,
				"lineStartAt": lineStartAt,
				"lineEndAt":   lineEndAt,
			} {
				got := fn(text, offset)
				if got < 0 || got > len(text) {
					t.Fatalf("%s(%q, %d) = %d, outside [0,%d]", name, text, offset, got, len(text))
				}
				_ = text[:got] // must not panic
			}

			for name, fn := range map[string]func(string, int) edit{
				"deleteWordLeft":    deleteWordLeft,
				"deleteToLineStart": deleteToLineStart,
			} {
				e := fn(text, offset)
				if e.start < 0 || e.end < e.start || e.end > len(text) {
					t.Fatalf("%s(%q, %d) produced range [%d,%d), outside [0,%d]",
						name, text, offset, e.start, e.end, len(text))
				}
				_ = applyEdit(text, e)
			}
		}
	}
}

// Offsets must land on character boundaries; a caret inside a multi-byte
// rune would corrupt the text on the next edit.
func TestMotionLandsOnCharacterBoundaries(t *testing.T) {
	const text = "SELECT 한글 FROM 테이블"

	for offset := 0; offset <= len(text); offset++ {
		for name, fn := range map[string]func(string, int) int{
			"wordLeft":    wordLeft,
			"wordRight":   wordRight,
			"lineStartAt": lineStartAt,
			"lineEndAt":   lineEndAt,
		} {
			got := fn(text, offset)
			if got < len(text) && !isCharBoundary(text, got) {
				t.Errorf("%s(%q, %d) = %d, which is inside a character", name, text, offset, got)
			}
		}
	}
}

// isCharBoundary reports whether an offset starts a UTF-8 sequence.
func isCharBoundary(s string, offset int) bool {
	return offset == 0 || offset == len(s) || s[offset]&0xC0 != 0x80
}
