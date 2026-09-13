package ui

import (
	"strings"
	"testing"
)

func baseTopBar() topBarState {
	return topBarState{
		dsName:  "prod-app",
		schema:  "app_db",
		helpKey: "F1",
	}
}

// The datasource is the last cue that says which server this is. It moved
// off the status bar precisely because that line sheds fields to fit, so on
// a narrow terminal the one thing that mattered was the one that went.
func TestTopBarNamesTheDatasourceAtEveryWidth(t *testing.T) {
	s := baseTopBar()

	for _, width := range []int{120, 80, 60, 40, 24, 12} {
		got, _ := s.renderWidth(width)

		if !strings.Contains(got, "prod-app") {
			t.Errorf("width %d: %q does not name the datasource", width, got)
		}
		if w := visibleWidth(got); w > width {
			t.Errorf("width %d: the bar is %d cells: %q", width, w, got)
		}
	}
}

// The chip is filled rather than merely coloured text, so it butts against
// the spine as one band of colour instead of reading as an error message.
func TestTheDatasourceChipIsFilledWithTheSpineColour(t *testing.T) {
	s := baseTopBar()

	want := colourTag(spineText, spineColour)
	if got, _ := s.renderWidth(80); !strings.Contains(got, want) {
		t.Errorf("%q does not carry the filled chip %q", got, want)
	}
}

// Which schema an unqualified statement reaches is a fact nothing else on
// screen says once the picker has closed.
func TestTopBarKeepsTheSchemaWhereverItFits(t *testing.T) {
	s := baseTopBar()

	for _, width := range []int{120, 80, 60, 40, 24} {
		if got, _ := s.renderWidth(width); !strings.Contains(got, "app_db") {
			t.Errorf("width %d: the schema was dropped: %q", width, got)
		}
	}
}

// A datasource is often named after its main schema, and two identical words
// side by side read as a repetition rather than as two facts.
func TestTheSchemaIsMarkedWithAnAt(t *testing.T) {
	got, _ := baseTopBar().renderWidth(120)
	if !strings.Contains(visibleText(got), "prod-app @app_db") {
		t.Errorf("%q does not join the datasource and the schema", got)
	}
}

func TestTopBarWithoutASchema(t *testing.T) {
	s := baseTopBar()
	s.schema = ""

	got, _ := s.renderWidth(120)
	if strings.Contains(got, "@") {
		t.Errorf("%q marks a schema when none is set", got)
	}
	if !strings.Contains(got, "prod-app") {
		t.Errorf("%q dropped the datasource along with the schema", got)
	}
}

// The order things go in is a judgement, and this is where it is stated: the
// help hint is a convenience, and the datasource is usually obvious from the
// context you opened it in.
func TestTopBarShedsTheHelpHintFirst(t *testing.T) {
	s := baseTopBar()

	wide, _ := s.renderWidth(120)
	if !strings.Contains(wide, "F1") {
		t.Fatalf("the help hint is missing at full width: %q", wide)
	}

	narrow, _ := s.renderWidth(24)
	if strings.Contains(narrow, "F1") {
		t.Errorf("width 24: the help hint survived: %q", narrow)
	}
	if !strings.Contains(narrow, "prod-app") {
		t.Errorf("width 24: something other than the hint went first: %q", narrow)
	}
}

// Names come from configuration, and can contain "[", which tview would read
// as the start of a colour tag and swallow.
func TestTopBarEscapesTagsInNames(t *testing.T) {
	s := baseTopBar()
	s.dsName = "db[1]"
	s.schema = "s[2]"

	got, _ := s.renderWidth(120)
	for _, want := range []string{"db[[1]", "s[[2]"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q does not escape %q", got, want)
		}
	}
}

// The line sheds fields until it fits. A zone that survived a form it was
// dropped from would put the datasource switcher under whatever moved into
// those columns.
func TestTopBarZonesAgreeWithEveryFormOfTheLine(t *testing.T) {
	state := topBarState{
		dsName:  "prod-app",
		schema:  "app_db",
		helpKey: "F1",
	}

	for _, width := range []int{120, 60, 40, 24, 12} {
		line, zones := state.renderWidth(width)
		plain := visibleText(line)

		for _, z := range zones {
			if z.from < 0 || z.to > len([]rune(plain)) || z.from >= z.to {
				t.Fatalf("width %d: zone %+v is outside the line %q", width, z, plain)
			}
			covered := string([]rune(plain)[z.from:z.to])
			switch z.target {
			case zoneDataSource:
				if !strings.Contains(covered, "prod-app") {
					t.Errorf("width %d: the datasource zone covers %q", width, covered)
				}
			case zoneSchema:
				if !strings.Contains(covered, "app_db") {
					t.Errorf("width %d: the schema zone covers %q", width, covered)
				}
			case zoneHelp:
				if !strings.Contains(covered, "F1") {
					t.Errorf("width %d: the help zone covers %q", width, covered)
				}
			}
		}
	}
}

func TestTheTopLineSaysWhenTheDataSourceIsReadOnly(t *testing.T) {
	line, _ := topBarState{dsName: "app", schema: "shop", readOnly: true}.renderWidth(80)
	if !strings.Contains(line, "read-only") {
		t.Errorf("renderWidth() = %q, want read-only on it", line)
	}
	line, _ = topBarState{dsName: "app", schema: "shop"}.renderWidth(80)
	if strings.Contains(line, "read-only") {
		t.Errorf("renderWidth() = %q says read-only for a datasource that is not", line)
	}
}

// The datasource name is chosen by whoever wrote the configuration file, and
// two of them can be one letter apart. The server and the account are the
// facts, and they are what someone is checking when they stop to ask whether
// this is the window they think it is.
func TestTheTopLineNamesTheServerAndTheAccount(t *testing.T) {
	s := baseTopBar()
	s.identity = "root@db.internal:3306"

	got, _ := s.renderWidth(120)
	if !strings.Contains(got, "root@db.internal:3306") {
		t.Errorf("renderWidth() = %q, want the account and the server on it", got)
	}
}

// What goes when the line will not hold everything, in order. The help key
// is a convenience; where you are is not, and the datasource, the schema and
// read-only are the three that must survive any terminal.
func TestTheTopLineShedsInOrderAndNeverShedsWhereYouAre(t *testing.T) {
	s := baseTopBar()
	s.identity = "root@db.internal:3306"
	s.readOnly = true

	wide, _ := s.renderWidth(120)
	if !strings.Contains(wide, "F1") || !strings.Contains(wide, "root@db.internal:3306") {
		t.Fatalf("width 120: %q does not carry everything", wide)
	}

	// Wide enough for the identity and not for the help key beside it: the
	// key is right-aligned and goes when the gap before it would close up.
	medium, _ := s.renderWidth(52)
	if strings.Contains(medium, "F1 keys") {
		t.Errorf("width 52: the help key outlasted the room for it: %q", medium)
	}
	if !strings.Contains(medium, "root@db.internal:3306") {
		t.Errorf("width 52: the server went before the help key: %q", medium)
	}

	// Too narrow for the identity at all. It goes rather than the line being
	// truncated, because truncating keeps the leftmost cells and read-only
	// is on the end.
	narrow, _ := s.renderWidth(40)
	if strings.Contains(narrow, "db.internal") {
		t.Errorf("width 40: the identity was kept and something else paid: %q", narrow)
	}

	for _, width := range []int{120, 52, 40, 30} {
		got, _ := s.renderWidth(width)
		for _, want := range []string{"prod-app", "app_db", "read-only"} {
			if !strings.Contains(got, want) {
				t.Errorf("width %d: %q lost %q", width, got, want)
			}
		}
		if w := visibleCost(got); w > width {
			t.Errorf("width %d: the bar is %d cells: %q", width, w, got)
		}
	}
}
