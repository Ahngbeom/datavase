package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/result"
)

func TestGridContentReservesRowZeroForTheHeader(t *testing.T) {
	buf := result.NewBuffer(0)
	buf.SetColumns([]string{"id", "email"}, nil)
	buf.Append([][]any{{int64(1), []byte("a@example.com")}})

	c := newGridContent(buf)

	// One data row plus the header row.
	if got := c.GetRowCount(); got != 2 {
		t.Errorf("GetRowCount() = %d, want 2", got)
	}
	if got := c.GetColumnCount(); got != 2 {
		t.Errorf("GetColumnCount() = %d, want 2", got)
	}
	if got := c.GetCell(0, 0).Text; got != "id" {
		t.Errorf("header cell = %q, want %q", got, "id")
	}
	if got := c.GetCell(1, 1).Text; got != "a@example.com" {
		t.Errorf("data cell = %q, want %q", got, "a@example.com")
	}
}

// An empty result still needs its header row, otherwise the grid shows
// nothing at all and the user cannot tell a failed query from an empty one.
func TestGridContentShowsHeadersForAnEmptyResult(t *testing.T) {
	buf := result.NewBuffer(0)
	buf.SetColumns([]string{"id"}, nil)

	c := newGridContent(buf)

	if got := c.GetRowCount(); got != 1 {
		t.Errorf("GetRowCount() = %d, want 1", got)
	}
	if got := c.GetCell(0, 0).Text; got != "id" {
		t.Errorf("header cell = %q, want %q", got, "id")
	}
}

func TestGridContentWithNoColumns(t *testing.T) {
	c := newGridContent(result.NewBuffer(0))

	if got := c.GetRowCount(); got != 0 {
		t.Errorf("GetRowCount() = %d, want 0", got)
	}
	if got := c.GetColumnCount(); got != 0 {
		t.Errorf("GetColumnCount() = %d, want 0", got)
	}
}

// tview requests cells for positions that may no longer exist by the time
// the request arrives. Returning a blank cell keeps the app alive.
func TestGridContentNeverReturnsNilForOutOfRangeCells(t *testing.T) {
	buf := result.NewBuffer(0)
	buf.SetColumns([]string{"id"}, nil)
	buf.Append([][]any{{int64(1)}})

	c := newGridContent(buf)

	positions := [][2]int{{-1, 0}, {0, -1}, {99, 0}, {0, 99}, {99, 99}}
	for _, p := range positions {
		cell := c.GetCell(p[0], p[1])
		if cell == nil {
			t.Errorf("GetCell(%d,%d) = nil, want a blank cell", p[0], p[1])
			continue
		}
		if cell.Text != "" {
			t.Errorf("GetCell(%d,%d).Text = %q, want empty", p[0], p[1], cell.Text)
		}
	}
}

// The row count has to follow the buffer as the stream fills it; that is
// what lets the user scroll through a result that is still arriving.
func TestGridContentTracksAGrowingBuffer(t *testing.T) {
	buf := result.NewBuffer(0)
	buf.SetColumns([]string{"n"}, nil)
	c := newGridContent(buf)

	if got := c.GetRowCount(); got != 1 {
		t.Fatalf("GetRowCount() = %d, want 1 before any rows", got)
	}

	buf.Append([][]any{{int64(1)}, {int64(2)}})

	if got := c.GetRowCount(); got != 3 {
		t.Errorf("GetRowCount() = %d, want 3 after two rows arrived", got)
	}
}
