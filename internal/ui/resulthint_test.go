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
//
// It also said nothing at all on the DDL, plan and sessions tabs. The fix is
// not this pure function alone: see returnToResults (app.go) and
// TestEscFromDDLReturnsFocusAndTabToResults (panel_integration_test.go),
// which drives the real tree-select-then-inspect path and checks focus, not
// just the string this test checks.
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
			name:  "the DDL tab is in front, where a user who clicked a table lands",
			state: resultState{tab: tabDDL},
			want:  "Esc returns to results",
		},
		{
			name:  "the plan tab is in front",
			state: resultState{tab: tabPlan, running: true},
			want:  "Esc returns to results",
		},
		{
			name:  "the sessions tab is in front",
			state: resultState{tab: tabSessions},
			want:  "Esc returns to results",
		},
		{
			name:  "the DDL tab is in front but a Tab press already moved focus into the editor",
			state: resultState{tab: tabDDL, editorFocused: true},
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
