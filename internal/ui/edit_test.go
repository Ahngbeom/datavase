package ui

import "testing"

// The edit helpers return a replacement for a byte range rather than a whole
// new buffer, because that is what TextArea.Replace takes — and using Replace
// is what keeps a single Ctrl+Z able to undo the edit.

func TestToggleCommentAddsToPlainLines(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		from, to   int
		wantText   string
		wantSelect string
	}{
		{
			name:     "single line",
			text:     "SELECT 1",
			from:     0,
			to:       0,
			wantText: "-- SELECT 1",
		},
		{
			name:     "cursor inside the line still comments the whole line",
			text:     "SELECT 1",
			from:     3,
			to:       3,
			wantText: "-- SELECT 1",
		},
		{
			name:     "indentation is preserved",
			text:     "    SELECT 1",
			from:     0,
			to:       0,
			wantText: "    -- SELECT 1",
		},
		{
			name:     "tab indentation is preserved",
			text:     "\tSELECT 1",
			from:     0,
			to:       0,
			wantText: "\t-- SELECT 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edit := toggleComment(tt.text, tt.from, tt.to)
			if got := applyEdit(tt.text, edit); got != tt.wantText {
				t.Errorf("toggleComment(%q) = %q, want %q", tt.text, got, tt.wantText)
			}
		})
	}
}

func TestToggleCommentAcrossSeveralLines(t *testing.T) {
	const text = "SELECT 1\nFROM t\nWHERE x = 1"

	edit := toggleComment(text, 0, len(text))
	got := applyEdit(text, edit)
	want := "-- SELECT 1\n-- FROM t\n-- WHERE x = 1"

	if got != want {
		t.Errorf("toggleComment() = %q, want %q", got, want)
	}
}

// Uncommenting has to restore the original exactly, or every toggle leaves
// the buffer a little different from where it started.
func TestToggleCommentRoundTrips(t *testing.T) {
	sources := []string{
		"SELECT 1",
		"    SELECT 1",
		"\tSELECT 1",
		"SELECT 1\nFROM t",
		"SELECT 1\n\nFROM t",
		"  SELECT 1\n    FROM t\n  WHERE x = 1",
	}

	for _, text := range sources {
		t.Run(text, func(t *testing.T) {
			commented := applyEdit(text, toggleComment(text, 0, len(text)))
			back := applyEdit(commented, toggleComment(commented, 0, len(commented)))

			if back != text {
				t.Errorf("round trip produced %q, want %q (commented form was %q)", back, text, commented)
			}
		})
	}
}

// A block where every line is already commented uncomments; a mixed block
// comments everything. This is what VS Code and DataGrip both do, and it
// stops "-- -- --" from accumulating.
func TestToggleCommentUsesTheAllCommentedRule(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "all commented uncomments",
			text: "-- SELECT 1\n-- FROM t",
			want: "SELECT 1\nFROM t",
		},
		{
			name: "mixed comments everything",
			text: "-- SELECT 1\nFROM t",
			want: "-- -- SELECT 1\n-- FROM t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyEdit(tt.text, toggleComment(tt.text, 0, len(tt.text)))
			if got != tt.want {
				t.Errorf("toggleComment(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// A comment marker without the conventional space is still a comment.
func TestToggleCommentRecognisesMarkerWithoutSpace(t *testing.T) {
	const text = "--SELECT 1"

	if got := applyEdit(text, toggleComment(text, 0, len(text))); got != "SELECT 1" {
		t.Errorf("toggleComment(%q) = %q, want %q", text, got, "SELECT 1")
	}
}

// Blank lines inside a selection are left alone; commenting them adds noise
// and blocks the round trip.
func TestToggleCommentSkipsBlankLines(t *testing.T) {
	const text = "SELECT 1\n\nFROM t"

	got := applyEdit(text, toggleComment(text, 0, len(text)))
	want := "-- SELECT 1\n\n-- FROM t"

	if got != want {
		t.Errorf("toggleComment(%q) = %q, want %q", text, got, want)
	}
}

func TestDuplicateLine(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		from, to int
		want     string
	}{
		{
			name: "single line",
			text: "SELECT 1",
			from: 0, to: 0,
			want: "SELECT 1\nSELECT 1",
		},
		{
			name: "middle line of three",
			text: "a\nb\nc",
			from: 2, to: 2,
			want: "a\nb\nb\nc",
		},
		{
			name: "last line",
			text: "a\nb",
			from: 2, to: 2,
			want: "a\nb\nb",
		},
		{
			name: "selection spanning two lines duplicates both",
			text: "a\nb\nc",
			from: 0, to: 3,
			want: "a\nb\na\nb\nc",
		},
		{
			name: "indentation is copied",
			text: "    SELECT 1",
			from: 0, to: 0,
			want: "    SELECT 1\n    SELECT 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyEdit(tt.text, duplicateLines(tt.text, tt.from, tt.to)); got != tt.want {
				t.Errorf("duplicateLines(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDeleteLine(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		from, to int
		want     string
	}{
		{
			name: "middle line",
			text: "a\nb\nc",
			from: 2, to: 2,
			want: "a\nc",
		},
		{
			name: "first line",
			text: "a\nb",
			from: 0, to: 0,
			want: "b",
		},
		{
			name: "last line takes the preceding newline with it",
			text: "a\nb",
			from: 2, to: 2,
			want: "a",
		},
		{
			name: "only line leaves an empty buffer",
			text: "a",
			from: 0, to: 0,
			want: "",
		},
		{
			name: "selection spanning two lines deletes both",
			text: "a\nb\nc",
			from: 0, to: 3,
			want: "c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyEdit(tt.text, deleteLines(tt.text, tt.from, tt.to)); got != tt.want {
				t.Errorf("deleteLines(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// The caret can sit outside the buffer while an edit is being applied, and a
// panic there would take the application down with the user's unsaved SQL.
func TestEditHelpersToleratePositionsOutsideTheBuffer(t *testing.T) {
	texts := []string{"", "a", "a\nb\nc", "한글\n테스트"}

	for _, text := range texts {
		for from := -3; from <= len(text)+3; from++ {
			for to := -3; to <= len(text)+3; to++ {
				for name, fn := range map[string]func(string, int, int) edit{
					"toggleComment":  toggleComment,
					"duplicateLines": duplicateLines,
					"deleteLines":    deleteLines,
				} {
					e := fn(text, from, to)
					if e.start < 0 || e.end < e.start || e.end > len(text) {
						t.Fatalf("%s(%q, %d, %d) produced range [%d,%d), outside [0,%d]",
							name, text, from, to, e.start, e.end, len(text))
					}
					_ = applyEdit(text, e) // must not panic
				}
			}
		}
	}
}

// Multi-byte text must not be cut mid-character.
func TestEditHelpersHandleMultibyteText(t *testing.T) {
	const text = "SELECT '한글'\nFROM 테이블"

	commented := applyEdit(text, toggleComment(text, 0, len(text)))
	want := "-- SELECT '한글'\n-- FROM 테이블"
	if commented != want {
		t.Errorf("toggleComment() = %q, want %q", commented, want)
	}

	if back := applyEdit(commented, toggleComment(commented, 0, len(commented))); back != text {
		t.Errorf("round trip = %q, want %q", back, text)
	}
}

// applyEdit is the test's stand-in for TextArea.Replace.
func applyEdit(text string, e edit) string {
	return text[:e.start] + e.text + text[e.end:]
}
