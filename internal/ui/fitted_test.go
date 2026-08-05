package ui

import "testing"

// A dialog was sized to its maximum rather than to what was in it: six lines
// of row detail sat in a box twenty-eight rows tall, and twenty-two empty rows
// inside a border read as a pane that is still loading.
func TestADialogIsAsTallAsWhatIsInIt(t *testing.T) {
	text := "id INT                  1\ncustomer_email VARCHAR  ada@example.com\nstatus VARCHAR          paid\n"

	// Three lines, plus the border above and below.
	if got := dialogHeight(text, 80); got != 5 {
		t.Errorf("dialogHeight() = %d, want 5", got)
	}
}

// A value too wide for the box wraps, and a box sized for the unwrapped line
// would cut the tail off with nothing to say it had.
func TestADialogCountsTheRowsAWrappedLineTakes(t *testing.T) {
	// Twenty visible cells inside a box ten wide, whose interior is eight.
	text := "aaaaaaaaaaaaaaaaaaaa\n"

	// Three wrapped rows, plus the border.
	if got := dialogHeight(text, 10); got != 5 {
		t.Errorf("dialogHeight() = %d, want 5", got)
	}
}

// Colour tags are markup, not cells, so a heavily tagged line must not be
// counted as though the reader could see the tags.
func TestADialogDoesNotCountColourTagsAsWidth(t *testing.T) {
	plain := dialogHeight("hello world\n", 20)
	tagged := dialogHeight(tag(colourAccent, "hello world")+"\n", 20)

	if plain != tagged {
		t.Errorf("a tagged line is %d rows and the same plain line is %d", tagged, plain)
	}
}

// An empty line is still a line.
func TestADialogCountsBlankLines(t *testing.T) {
	if got := dialogHeight("one\n\nthree\n", 80); got != 5 {
		t.Errorf("dialogHeight() = %d, want 5", got)
	}
}

// A trailing newline ends the last line rather than starting another; a box
// with a spare row under the last one looks like something failed to render.
func TestADialogDoesNotCountATrailingNewlineAsALine(t *testing.T) {
	with := dialogHeight("one\ntwo\n", 80)
	without := dialogHeight("one\ntwo", 80)

	if with != without {
		t.Errorf("a trailing newline changed the height from %d to %d", without, with)
	}
}
