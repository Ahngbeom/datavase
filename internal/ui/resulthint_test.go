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
			state: resultState{tab: tabResults},
			want:  "run a statement to see rows here",
		},
		{
			name:  "a statement is running",
			state: resultState{tab: tabResults, running: true},
			want:  "waiting for the first row…",
		},
		{
			name:  "the statement changed rows rather than returning them",
			state: resultState{tab: tabResults, wrote: true},
			want:  "no rows: that statement changed data",
		},
		{
			name:  "rows arrived",
			state: resultState{tab: tabResults, columns: 3},
			want:  "",
		},
		{
			name:  "rows are arriving now",
			state: resultState{tab: tabResults, columns: 3, running: true},
			want:  "",
		},
		{
			name:  "another tab is in front",
			state: resultState{tab: tabPlan, running: true},
			want:  "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultHint(tt.state); got != tt.want {
				t.Errorf("resultHint() = %q, want %q", got, tt.want)
			}
		})
	}
}
