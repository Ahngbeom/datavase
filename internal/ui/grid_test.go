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

// sorted builds a grid over one text column, in the order given.
func sortedGrid(t *testing.T, values ...string) (*result.Buffer, *gridContent) {
	t.Helper()

	buf := result.NewBuffer(0)
	buf.SetColumns([]string{"name"}, nil)
	rows := make([][]any, len(values))
	for i, v := range values {
		rows[i] = []any{[]byte(v)}
	}
	buf.Append(rows)

	return buf, newGridContent(buf)
}

// displayed reads the column down the grid, which is the order a user sees.
func displayed(c *gridContent) []string {
	out := make([]string, 0, c.GetRowCount()-1)
	for row := 1; row < c.GetRowCount(); row++ {
		out = append(out, c.GetCell(row, 0).Text)
	}
	return out
}

func TestSortingCyclesThroughAscendingDescendingAndBack(t *testing.T) {
	_, c := sortedGrid(t, "charlie", "alpha", "bravo")

	if got := displayed(c); !equalStrings(got, []string{"charlie", "alpha", "bravo"}) {
		t.Fatalf("unsorted grid shows %v, want the buffer's own order", got)
	}

	c.sortBy(0)
	if got := displayed(c); !equalStrings(got, []string{"alpha", "bravo", "charlie"}) {
		t.Errorf("ascending shows %v", got)
	}

	c.sortBy(0)
	if got := displayed(c); !equalStrings(got, []string{"charlie", "bravo", "alpha"}) {
		t.Errorf("descending shows %v", got)
	}

	// The third press is the one a grid usually leaves out, and the only way
	// back to the order the server sent — which is the answer to an ORDER BY
	// that was in the statement.
	c.sortBy(0)
	if got := displayed(c); !equalStrings(got, []string{"charlie", "alpha", "bravo"}) {
		t.Errorf("the third press shows %v, want the buffer's own order back", got)
	}
	if c.sorted() {
		t.Error("the grid still reports itself sorted after cycling back")
	}
}

// Everything that acts on "the selected row" — copying it, opening it, saying
// which of how many it is — reads the grid's row number. Once sorted, that is
// no longer the buffer's row number, and one place has to say so.
func TestASortedGridMapsItsRowsBackToTheBuffer(t *testing.T) {
	_, c := sortedGrid(t, "charlie", "alpha", "bravo")

	if got := c.bufferRow(2); got != 1 {
		t.Errorf("unsorted, grid row 2 is buffer row %d, want 1", got)
	}

	c.sortBy(0)

	// alpha, bravo, charlie on screen; 1, 2, 0 in the buffer.
	for gridRow, want := range map[int]int{1: 1, 2: 2, 3: 0} {
		if got := c.bufferRow(gridRow); got != want {
			t.Errorf("grid row %d is buffer row %d, want %d", gridRow, got, want)
		}
	}
}

// A result is still arriving while it can be sorted, so an order built over
// the rows received so far is stale the moment more land.
func TestSortingFollowsRowsThatArriveAfterIt(t *testing.T) {
	buf, c := sortedGrid(t, "charlie", "alpha")

	c.sortBy(0)
	if got := displayed(c); !equalStrings(got, []string{"alpha", "charlie"}) {
		t.Fatalf("ascending shows %v", got)
	}

	buf.Append([][]any{{[]byte("bravo")}})

	if got := displayed(c); !equalStrings(got, []string{"alpha", "bravo", "charlie"}) {
		t.Errorf("after a row arrived the grid shows %v, want it sorted in", got)
	}
}

// Sorting a column and then running something else must not carry the old
// column's ordering onto a result that may not even have that many columns.
func TestANewResultIsNotStillSortedByTheLastOne(t *testing.T) {
	buf, c := sortedGrid(t, "charlie", "alpha")
	c.sortBy(0)

	buf.Reset()
	c.unsort()
	buf.SetColumns([]string{"n"}, nil)
	buf.Append([][]any{{[]byte("2")}, {[]byte("1")}})

	if got := displayed(c); !equalStrings(got, []string{"2", "1"}) {
		t.Errorf("the new result shows %v, want the order it arrived in", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Sorting the rows in hand is not sorting the query's answer, and on screen the
// two are identical. Both ways that can happen have to be said out loud.
func TestSortNoticeAdmitsWhenTheRowsAreNotTheResult(t *testing.T) {
	for _, tt := range []struct {
		name  string
		state sortState
		want  string
	}{
		{
			name:  "a complete result just says what it did",
			state: sortState{column: "id", rows: 40},
			want:  "sorted by id, ascending",
		},
		{
			name:  "and which way",
			state: sortState{column: "id", descending: true, rows: 40},
			want:  "sorted by id, descending",
		},
		{
			// The rows missing from a cut result are exactly the ones that
			// might have sorted to the top, so this is the case where the
			// screen is most convincing and least right.
			name:  "a cut result says the ordering is not the whole one",
			state: sortState{column: "id", rows: 50000, atCapacity: true},
			want:  "sorted the 50000 rows kept by id, ascending — the result was cut, so this is not the whole ordering",
		},
		{
			name:  "a result still arriving says so",
			state: sortState{column: "id", rows: 500, arriving: true},
			want:  "sorted the 500 rows so far by id, ascending — more are still arriving",
		},
		{
			// Cut beats still-arriving: a result that hit capacity has stopped
			// taking rows, so "more are still arriving" would be the less true
			// of the two.
			name:  "cut is reported ahead of arriving",
			state: sortState{column: "id", rows: 50000, atCapacity: true, arriving: true},
			want:  "sorted the 50000 rows kept by id, ascending — the result was cut, so this is not the whole ordering",
		},
		{
			name:  "no column means the ordering was dropped",
			state: sortState{rows: 40},
			want:  "back to the order the rows arrived in",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortNotice(tt.state); got != tt.want {
				t.Errorf("sortNotice() = %q,\n            want %q", got, tt.want)
			}
		})
	}
}
