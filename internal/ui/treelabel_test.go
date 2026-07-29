package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

func testDataSource() *config.DataSource {
	return &config.DataSource{
		Name:     "acme-dev",
		Env:      config.EnvDev,
		Host:     "db.internal",
		Port:     6446,
		Database: "acme-dev",
	}
}

// The reported confusion: the datasource is named the same as one of its
// schemas, so the root read as a schema and everything under it looked like
// a child schema. The host is what tells them apart.
func TestRootLabelNamesTheServer(t *testing.T) {
	got := rootLabel(testDataSource(), 60)

	if !strings.Contains(got, "acme-dev") {
		t.Errorf("rootLabel() = %q, want it to name the datasource", got)
	}
	if !strings.Contains(got, "db.internal") {
		t.Errorf("rootLabel() = %q, want it to name the host", got)
	}
	if !strings.Contains(got, "6446") {
		t.Errorf("rootLabel() = %q, want it to name the port", got)
	}
}

// A root label and a schema of the same name must never render identically —
// that identity is the whole bug.
func TestRootLabelDiffersFromASchemaOfTheSameName(t *testing.T) {
	ds := testDataSource()

	root := rootLabel(ds, 60)
	schema := schemaLabel(ds.Database, ds.Database)

	if root == schema {
		t.Errorf("the root and the schema both render as %q", root)
	}
}

// Narrow panes must keep the host: without it the root is just a name again,
// which is the state being fixed. The datasource name gives way instead.
func TestRootLabelKeepsTheHostWhenSpaceIsTight(t *testing.T) {
	ds := testDataSource()
	ds.Name = "a-very-long-datasource-name-that-will-not-fit"

	for _, width := range []int{40, 30, 24, 18} {
		got := rootLabel(ds, width)

		if len([]rune(got)) > width {
			t.Errorf("width %d: rootLabel() = %q, which is %d runes",
				width, got, len([]rune(got)))
		}
		if !strings.Contains(got, "db.internal") {
			t.Errorf("width %d: rootLabel() = %q, want the host kept", width, got)
		}
	}
}

// With nothing but the host left, it still has to fit rather than overflow.
func TestRootLabelSurvivesAnAbsurdlyNarrowPane(t *testing.T) {
	for _, width := range []int{10, 5, 1, 0, -1} {
		got := rootLabel(testDataSource(), width)

		if width > 0 && len([]rune(got)) > width {
			t.Errorf("width %d: rootLabel() = %q, too long", width, got)
		}
		if got == "" {
			t.Errorf("width %d: rootLabel() is empty", width)
		}
	}
}

// The default schema is marked, because nothing else says which one an
// unqualified query will hit.
func TestSchemaLabelMarksTheCurrentSchema(t *testing.T) {
	current := schemaLabel("acme-dev", "acme-dev")
	other := schemaLabel("common", "acme-dev")

	if current == other {
		t.Fatal("the current schema renders the same as any other")
	}
	if !strings.Contains(current, "acme-dev") {
		t.Errorf("schemaLabel() = %q, want it to contain the name", current)
	}
	if !strings.Contains(other, "common") {
		t.Errorf("schemaLabel() = %q, want it to contain the name", other)
	}
	// The marker belongs to the current one only.
	if strings.Contains(other, currentSchemaMarker) {
		t.Errorf("a non-current schema carries the marker: %q", other)
	}
}

// With no default schema configured, nothing is marked.
func TestSchemaLabelWithoutACurrentSchema(t *testing.T) {
	got := schemaLabel("common", "")

	if strings.Contains(got, currentSchemaMarker) {
		t.Errorf("schemaLabel() = %q, want no marker when no schema is current", got)
	}
}

// Names come from the server and can contain tview's tag characters.
func TestLabelsEscapeTagCharacters(t *testing.T) {
	ds := testDataSource()
	ds.Name = "ds[1]"
	ds.Host = "host[2]"

	if got := rootLabel(ds, 60); strings.Contains(got, "[1]") && !strings.Contains(got, "[[1]") {
		t.Errorf("rootLabel() = %q, want the tag characters escaped", got)
	}
	if got := schemaLabel("sch[3]", ""); strings.Contains(got, "[3]") && !strings.Contains(got, "[[3]") {
		t.Errorf("schemaLabel() = %q, want the tag characters escaped", got)
	}
}
