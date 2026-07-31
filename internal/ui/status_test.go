package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
)

func baseStatus() status {
	return status{dsName: "prod-app", env: config.EnvProd}
}

// The environment badge is the last visual cue between the user and a
// production mistake, so it must always be present and always be red.
func TestStatusAlwaysShowsTheEnvironmentBadge(t *testing.T) {
	tests := []struct {
		env    config.Env
		colour string
	}{
		{env: config.EnvProd, colour: "red"},
		{env: config.EnvStage, colour: "yellow"},
		{env: config.EnvDev, colour: "green"},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			s := baseStatus()
			s.env = tt.env

			for _, phase := range []runPhase{phaseIdle, phaseRunning, phaseDone, phaseFailed} {
				s.phase = phase
				got := s.render()

				if !strings.Contains(got, string(tt.env)) {
					t.Errorf("phase %v: render() = %q, want it to name the environment", phase, got)
				}
				if !strings.Contains(got, tt.colour) {
					t.Errorf("phase %v: render() = %q, want the %s badge colour", phase, got, tt.colour)
				}
			}
		})
	}
}

// Without this, there is no way to tell which schema an unqualified query
// will hit — the tree marker only helps if you scroll to it.
// The status bar is one line on a terminal of unknown width. When it does
// not fit, what disappears must be chosen deliberately: an elapsed time is
// expendable, a truncation warning is not.
func TestStatusDropsExpendableFieldsBeforeWarnings(t *testing.T) {
	s := baseStatus()
	s.schema = "acme-dev"
	s.phase = phaseDone
	s.rows = 50000
	s.elapsed = 1234 * time.Millisecond
	s.limitInjected = 1000
	s.truncated = true

	for _, width := range []int{120, 100, 80, 60, 50, 40} {
		got := s.renderWidth(width)

		if w := visibleWidth(got); w > width {
			t.Errorf("width %d: status is %d cells: %q", width, w, got)
		}
		// The environment and the warnings survive at every width.
		if !strings.Contains(got, string(config.EnvProd)) {
			t.Errorf("width %d: the environment badge was dropped: %q", width, got)
		}
		if !strings.Contains(strings.ToUpper(got), "LIMIT") {
			t.Errorf("width %d: the injected LIMIT warning was dropped: %q", width, got)
		}
		if !strings.Contains(strings.ToLower(got), "truncated") {
			t.Errorf("width %d: the truncation warning was dropped: %q", width, got)
		}
	}
}

// An error is the other thing that must never be squeezed out.
func TestStatusKeepsTheErrorAtAnyWidth(t *testing.T) {
	s := baseStatus()
	s.schema = "acme-dev"
	s.phase = phaseFailed
	s.err = errors.New("Error 1064: syntax")

	for _, width := range []int{120, 80, 50, 40} {
		got := s.renderWidth(width)
		if !strings.Contains(got, "1064") {
			t.Errorf("width %d: the server error was dropped: %q", width, got)
		}
	}
}

// visibleWidth measures what the terminal shows, ignoring colour tags.
func visibleWidth(s string) int {
	var (
		count int
		inTag bool
		runes = []rune(s)
	)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '[' && i+1 < len(runes) && runes[i+1] == '[':
			count++
			i++
		case runes[i] == '[':
			inTag = true
		case runes[i] == ']' && inTag:
			inTag = false
		case !inTag:
			count++
		}
	}
	return count
}

func TestStatusShowsTheCurrentSchema(t *testing.T) {
	s := baseStatus()
	s.schema = "acme-dev"

	got := s.render()
	if !strings.Contains(got, "acme-dev") {
		t.Errorf("render() = %q, want it to name the current schema", got)
	}
	// The datasource is also called acme-dev here, so the schema needs a
	// marker of its own to be legible.
	if !strings.Contains(got, "@acme-dev") {
		t.Errorf("render() = %q, want the schema marked with @", got)
	}
}

func TestStatusWithoutASchema(t *testing.T) {
	got := baseStatus().render()

	if strings.Contains(got, "@") {
		t.Errorf("render() = %q, want no schema marker when none is set", got)
	}
}

func TestStatusShowsTheDataSourceName(t *testing.T) {
	if got := baseStatus().render(); !strings.Contains(got, "prod-app") {
		t.Errorf("render() = %q, want it to name the datasource", got)
	}
}

func TestStatusWhileRunning(t *testing.T) {
	s := baseStatus()
	s.phase = phaseRunning

	got := s.render()
	if !strings.Contains(strings.ToLower(got), "running") {
		t.Errorf("render() = %q, want it to say the query is running", got)
	}
	if !strings.Contains(got, "^C") {
		t.Errorf("render() = %q, want it to advertise the cancel key", got)
	}
}

func TestStatusOnSuccessReportsRowsAndTime(t *testing.T) {
	s := baseStatus()
	s.phase = phaseDone
	s.rows = 1234
	s.elapsed = 14 * time.Millisecond

	got := s.render()
	if !strings.Contains(got, "1234") {
		t.Errorf("render() = %q, want the row count", got)
	}
	if !strings.Contains(got, "14ms") {
		t.Errorf("render() = %q, want the elapsed time", got)
	}
}

// A silently added LIMIT would make a partial result look complete, so the
// status bar has to admit it every time.
func TestStatusDisclosesAnInjectedLimit(t *testing.T) {
	s := baseStatus()
	s.phase = phaseDone
	s.rows = 1000
	s.limitInjected = 1000

	got := s.render()
	if !strings.Contains(strings.ToUpper(got), "LIMIT 1000") {
		t.Errorf("render() = %q, want it to disclose the injected LIMIT", got)
	}
}

func TestStatusReportsTruncation(t *testing.T) {
	s := baseStatus()
	s.phase = phaseDone
	s.rows = 50000
	s.truncated = true

	if got := s.render(); !strings.Contains(strings.ToLower(got), "truncated") {
		t.Errorf("render() = %q, want it to report truncation", got)
	}
}

func TestStatusReportsFailure(t *testing.T) {
	s := baseStatus()
	s.phase = phaseFailed
	s.err = errors.New("Error 1064: You have an error in your SQL syntax")

	got := s.render()
	if !strings.Contains(got, "1064") {
		t.Errorf("render() = %q, want it to carry the server message", got)
	}
}

// A multi-line driver error would push the layout apart; the status bar is
// one line by contract.
func TestStatusRendersOnASingleLine(t *testing.T) {
	s := baseStatus()
	s.phase = phaseFailed
	s.err = errors.New("line one\nline two\nline three")

	if got := s.render(); strings.Contains(got, "\n") {
		t.Errorf("render() = %q, want a single line", got)
	}
}

// The write lock is a mode the user can forget they turned on.
func TestStatusShowsWhenProductionWritesAreUnlocked(t *testing.T) {
	locked := baseStatus()
	if got := locked.render(); strings.Contains(strings.ToLower(got), "writes on") {
		t.Errorf("render() = %q, want no write indicator while locked", got)
	}

	unlocked := baseStatus()
	unlocked.writesEnabled = true
	if got := unlocked.render(); !strings.Contains(strings.ToLower(got), "writes on") {
		t.Errorf("render() = %q, want it to warn that writes are unlocked", got)
	}
}

func TestStatusShowsAMessage(t *testing.T) {
	s := baseStatus()
	s.message = "cancelled"

	if got := s.render(); !strings.Contains(got, "cancelled") {
		t.Errorf("render() = %q, want it to include the message", got)
	}
}

// Values coming from the server can contain "[", which tview would read as
// a colour tag and swallow.
func TestStatusEscapesTagsInDynamicText(t *testing.T) {
	s := baseStatus()
	s.dsName = "db[1]"
	s.phase = phaseFailed
	s.err = errors.New("bad [thing]")

	got := s.render()
	if !strings.Contains(got, "db[[1]") {
		t.Errorf("render() = %q, want the datasource name escaped", got)
	}
	if !strings.Contains(got, "[[thing]") {
		t.Errorf("render() = %q, want the error text escaped", got)
	}
}

// On a modal keyboard the mode is the difference between "this key does
// nothing" and "this key does something else". It is never dropped, and it
// sits at the front where a truncated line still shows it.
func TestStatusShowsTheVimMode(t *testing.T) {
	s := baseStatus()
	s.vimMode = "NORMAL"

	got := s.render()
	if !strings.Contains(got, "NORMAL") {
		t.Errorf("render() = %q, want it to name the mode", got)
	}

	for _, width := range []int{120, 80, 40, 20} {
		if narrow := s.renderWidth(width); !strings.Contains(narrow, "NORMAL") {
			t.Errorf("width %d: the mode was dropped: %q", width, narrow)
		}
	}
}

// A half-typed sequence must be visible, or pressing d and seeing nothing
// happen reads as a broken keyboard.
func TestStatusShowsAPendingSequence(t *testing.T) {
	s := baseStatus()
	s.vimMode = "NORMAL"
	s.vimPending = "d"

	got := s.render()
	if !strings.Contains(got, "d") {
		t.Errorf("render() = %q, want the pending sequence", got)
	}

	for _, width := range []int{120, 60, 30} {
		if narrow := s.renderWidth(width); !strings.Contains(narrow, "NORMAL") {
			t.Errorf("width %d: the mode was dropped: %q", width, narrow)
		}
	}
}

func TestStatusWithoutAModalKeyboard(t *testing.T) {
	if got := baseStatus().render(); strings.Contains(got, "NORMAL") {
		t.Errorf("render() = %q, want no mode on a non-modal keyboard", got)
	}
}

// A batch that stopped part-way has left the database in a state nothing on
// screen describes, and there is no transaction to unwind it. The count of
// what actually ran is the only thing that tells the user where to look, so
// it is reported whether or not anything went wrong.
func TestABatchAlwaysSaysHowManyStatementsRan(t *testing.T) {
	tests := []struct {
		name             string
		total, ran       int
		why              string
		wantAll, wantNot []string
	}{
		{
			name:    "every statement ran",
			total:   5,
			ran:     5,
			wantAll: []string{"5 statements", "5 ran"},
		},
		{
			name:    "refused part-way",
			total:   5,
			ran:     2,
			why:     "refused at statement 3",
			wantAll: []string{"5 statements", "2 ran", "refused at statement 3"},
		},
		{
			name:    "the first statement was refused",
			total:   4,
			ran:     0,
			why:     "refused at statement 1",
			wantAll: []string{"0 ran", "refused at statement 1"},
			// "0 ran" has to be said, not left to be inferred from silence.
			wantNot: []string{"1 ran"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := batchSummary(tt.total, tt.ran, tt.why, false)
			for _, want := range tt.wantAll {
				if !strings.Contains(got, want) {
					t.Errorf("batchSummary() = %q, want it to contain %q", got, want)
				}
			}
			for _, unwanted := range tt.wantNot {
				if strings.Contains(got, unwanted) {
					t.Errorf("batchSummary() = %q, want it not to contain %q", got, unwanted)
				}
			}
		})
	}
}

// This is the number that says whether a write went as intended, and it is
// read in a hurry, so the singular is spelled out rather than left as
// "1 rows affected".
func TestAWriteReportsWhatItChangedRatherThanARowCount(t *testing.T) {
	tests := []struct {
		name             string
		written          db.Result
		wantAll, wantNot []string
	}{
		{
			name:    "one row",
			written: db.Result{RowsAffected: 1},
			wantAll: []string{"1 row affected"},
			wantNot: []string{"1 rows"},
		},
		{
			name:    "many rows",
			written: db.Result{RowsAffected: 4812},
			wantAll: []string{"4812 rows affected"},
		},
		{
			// A write that matched rows but changed none reports zero, which
			// is the server's own answer. Saying "affected" rather than
			// "matched" is what stops it being read as the other number.
			name:    "nothing changed",
			written: db.Result{RowsAffected: 0},
			wantAll: []string{"0 rows affected"},
		},
		{
			name:    "an insert carries the id it was given",
			written: db.Result{RowsAffected: 1, LastInsertID: 42},
			wantAll: []string{"1 row affected", "42"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			written := tt.written
			s := status{phase: phaseDone, written: &written}
			got := s.renderWidth(200)

			for _, want := range tt.wantAll {
				if !strings.Contains(got, want) {
					t.Errorf("status = %q, want it to contain %q", got, want)
				}
			}
			for _, unwanted := range tt.wantNot {
				if strings.Contains(got, unwanted) {
					t.Errorf("status = %q, want it not to contain %q", got, unwanted)
				}
			}
		})
	}
}

// A statement that returned rows has no count to report, and "0 rows
// affected" under a SELECT would be a number meaning nothing.
func TestAQueryStillReportsItsRowCount(t *testing.T) {
	s := status{phase: phaseDone, rows: 7}

	got := s.renderWidth(200)
	if !strings.Contains(got, "7 rows") {
		t.Errorf("status = %q, want the row count", got)
	}
	if strings.Contains(got, "affected") {
		t.Errorf("status = %q, want no affected-row count for a query", got)
	}
}

// A warning is the only sign that a statement the server called a success did
// something other than what was asked, so it says what happened rather than
// only that something did — and it survives a narrow terminal.
func TestAWarningReachesTheBarWithItsMessage(t *testing.T) {
	s := status{
		phase:   phaseDone,
		written: &db.Result{RowsAffected: 1},
		warnings: []db.Warning{
			{Level: "Warning", Code: 1265, Message: "Data truncated for column 's' at row 1"},
		},
	}

	got := s.renderWidth(200)
	if !strings.Contains(got, "1 warning") {
		t.Errorf("status = %q, want the warning count", got)
	}
	if !strings.Contains(got, "Data truncated for column 's' at row 1") {
		t.Errorf("status = %q, want the server's own message", got)
	}
}

// Dropping the warning to make the line fit would lose the only notice that
// the data is not what was asked for. The elapsed time may go instead.
func TestANarrowBarKeepsTheWarningAndDropsTheTiming(t *testing.T) {
	s := status{
		dsName:  "prod-app",
		schema:  "app_db",
		phase:   phaseDone,
		rows:    3,
		elapsed: 1234 * time.Millisecond,
		warnings: []db.Warning{
			{Level: "Warning", Code: 1265, Message: "Data truncated"},
		},
	}

	got := s.renderWidth(46)
	if !strings.Contains(got, "warning") {
		t.Errorf("status = %q, want the warning to survive the squeeze", got)
	}
}

func TestNoWarningsAddNothingToTheBar(t *testing.T) {
	s := status{phase: phaseDone, rows: 3}

	if got := s.renderWidth(200); strings.Contains(got, "warning") {
		t.Errorf("status = %q, want no mention of warnings", got)
	}
}

// The count exists because a part-way batch could not be taken back. Inside a
// transaction it still can, and a summary that read the same either way would
// leave the reader to guess the one thing that decides what to do next.
func TestABatchInsideATransactionSaysTheWorkCanStillBeTakenBack(t *testing.T) {
	outside := batchSummary(5, 2, "refused at statement 3", false)
	inside := batchSummary(5, 2, "refused at statement 3", true)

	if strings.Contains(outside, "rollback") {
		t.Errorf("outside a transaction: %q offers a rollback that does not exist", outside)
	}
	if !strings.Contains(inside, "rollback") {
		t.Errorf("inside a transaction: %q does not say the work can be undone", inside)
	}
}
