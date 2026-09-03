package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

func baseTopBar() topBarState {
	return topBarState{
		env:     config.EnvProd,
		dsName:  "prod-app",
		schema:  "app_db",
		helpKey: "F1",
	}
}

// The environment is the last cue between the user and a production mistake.
// It moved off the status bar precisely because that line sheds fields to fit,
// so on a narrow terminal the one thing that mattered was the one that went.
func TestTopBarNamesTheEnvironmentAtEveryWidth(t *testing.T) {
	for _, env := range []config.Env{config.EnvProd, config.EnvStage, config.EnvDev} {
		s := baseTopBar()
		s.env = env
		s.branch = "feature/a-rather-long-branch-name"

		for _, width := range []int{120, 80, 60, 40, 24, 12} {
			got, _ := s.renderWidth(width)

			if !strings.Contains(strings.ToLower(got), string(env)) {
				t.Errorf("%s at width %d: %q does not name the environment", env, width, got)
			}
			if w := visibleWidth(got); w > width {
				t.Errorf("%s at width %d: the bar is %d cells: %q", env, width, w, got)
			}
		}
	}
}

// Production is red, and it is red as a filled chip rather than as red text.
// An error is red text; the two must not be the same thing worn twice.
func TestTheEnvironmentChipIsFilledWithTheEnvironmentColour(t *testing.T) {
	for env, want := range map[config.Env]string{
		config.EnvProd:  colourTag(spineTextLoud, spineProd),
		config.EnvStage: colourTag(spineTextLoud, spineStage),
		config.EnvDev:   colourTag(spineTextQuiet, spineDev),
	} {
		s := baseTopBar()
		s.env = env

		if got, _ := s.renderWidth(80); !strings.Contains(got, want) {
			t.Errorf("%s: %q does not carry the filled chip %q", env, got, want)
		}
	}
}

// Which schema an unqualified statement reaches is the other fact a production
// mistake is made of, and nothing else on screen says it once the picker has
// closed.
func TestTopBarKeepsTheSchemaWhereverItFits(t *testing.T) {
	s := baseTopBar()
	s.branch = "feature/a-rather-long-branch-name"

	for _, width := range []int{120, 80, 60, 40, 24} {
		if got, _ := s.renderWidth(width); !strings.Contains(got, "app_db") {
			t.Errorf("width %d: the schema was dropped: %q", width, got)
		}
	}
}

// A datasource is often named after its main schema, and two identical words
// side by side read as a repetition rather than as two facts.
func TestTheSchemaIsMarkedWithAnAt(t *testing.T) {
	if got, _ := baseTopBar().renderWidth(120); !strings.Contains(got, "prod-app@app_db") {
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
// help hint is a convenience, the datasource is usually obvious from the
// context you opened it in, and the branch says which piece of work these
// files belong to.
func TestTopBarShedsTheHelpHintFirst(t *testing.T) {
	s := baseTopBar()
	s.branch = "feature/add-index"

	wide, _ := s.renderWidth(120)
	if !strings.Contains(wide, "F1") {
		t.Fatalf("the help hint is missing at full width: %q", wide)
	}

	narrow, _ := s.renderWidth(46)
	if strings.Contains(narrow, "F1") {
		t.Errorf("width 46: the help hint survived: %q", narrow)
	}
	if !strings.Contains(narrow, "prod-app") || !strings.Contains(narrow, "feature/add-index") {
		t.Errorf("width 46: something other than the hint went first: %q", narrow)
	}
}

func TestTopBarShedsTheDataSourceBeforeTheBranch(t *testing.T) {
	s := baseTopBar()
	s.branch = "feature/add-index"

	// Wide enough for the branch once the datasource is gone, and not wide
	// enough for both.
	const width = 38

	got, _ := s.renderWidth(width)
	if strings.Contains(got, "prod-app") {
		t.Errorf("width %d: the datasource survived: %q", width, got)
	}
	if !strings.Contains(got, "feature/add-index") {
		t.Errorf("width %d: the branch went before the datasource: %q", width, got)
	}
	if !strings.Contains(got, "@app_db") {
		t.Errorf("width %d: the schema was dropped: %q", width, got)
	}
}

// Names come from configuration and from git, and either can contain "[",
// which tview would read as the start of a colour tag and swallow.
func TestTopBarEscapesTagsInNames(t *testing.T) {
	s := baseTopBar()
	s.dsName = "db[1]"
	s.schema = "s[2]"
	s.branch = "b[3]"

	got, _ := s.renderWidth(120)
	for _, want := range []string{"db[[1]", "s[[2]", "b[[3]"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q does not escape %q", got, want)
		}
	}
}

// The bar is one line by contract; the layout gives it exactly one row.
func TestTopBarRendersOnASingleLine(t *testing.T) {
	s := baseTopBar()
	s.branch = "feature/one\nfeature/two"

	if got, _ := s.renderWidth(120); strings.Contains(got, "\n") {
		t.Errorf("%q is not a single line", got)
	}
}

// With no worktree attached there is no branch, and an empty field would leave
// a separator with nothing after it.
func TestTopBarWithoutABranch(t *testing.T) {
	got, _ := baseTopBar().renderWidth(120)

	if strings.Contains(got, "·") {
		t.Errorf("%q leaves a separator with nothing after it", got)
	}
}

// The line sheds fields until it fits. A zone that survived a form it was
// dropped from would put the datasource switcher under whatever moved into
// those columns.
func TestTopBarZonesAgreeWithEveryFormOfTheLine(t *testing.T) {
	state := topBarState{
		env:     config.EnvProd,
		dsName:  "prod-app",
		schema:  "app_db",
		branch:  "feature/add-index",
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

// The environment is a warning, not a control: a misclick on the production
// marker must not be able to look like it changed the environment.
func TestTheEnvironmentIsNotClickable(t *testing.T) {
	state := topBarState{env: config.EnvProd, dsName: "prod-app", schema: "app_db"}

	_, zones := state.renderWidth(120)
	for _, z := range zones {
		if z.from == 0 {
			t.Errorf("a zone starts in the environment chip: %+v", z)
		}
	}
}
