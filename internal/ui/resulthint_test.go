package ui

import "testing"

// The results tab has no box around it, so an empty one is simply blank —
// which reads as a gap in the layout rather than as a pane waiting for a
// statement. The header carries a line saying which it is.
//
// That line only knew one thing, so it went on inviting the user to run a
// statement while one was running: the bar said "running… ^C cancels" and the
// pane two rows above said "run a statement to see rows here", at the moment
// the user is watching the screen hardest.
func TestTheResultHint(t *testing.T) {
	for _, tt := range []struct {
		name  string
		state resultState
		want  string
	}{
		{
			name:  "nothing has been run",
			state: resultState{},
			want:  "⌘↩ runs the statement",
		},
		{
			name:  "a statement is running",
			state: resultState{running: true},
			want:  "waiting for the first row…",
		},
		{
			name:  "the statement changed rows rather than returning them",
			state: resultState{wrote: true},
			want:  "no rows: that statement changed data",
		},
		{
			name:  "rows arrived",
			state: resultState{columns: 3},
			want:  "",
		},
		{
			name:  "rows are arriving now",
			state: resultState{columns: 3, running: true},
			want:  "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultHint(tt.state, testKeys()); got != tt.want {
				t.Errorf("resultHint() = %q, want %q", got, tt.want)
			}
		})
	}
}
