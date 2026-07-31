//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// The reported bug: with the datasource named after one of its schemas, the
// root looked like a schema and the real schemas looked nested under it.
func TestTreeRootIsDistinguishableFromASchemaOfTheSameName(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	got := h.text()

	// The root carries the host, which no schema node does.
	if !strings.Contains(got, testmysql.DefaultHost) {
		t.Errorf("the tree root does not name the server:\n%s", got)
	}
}

// The default schema is the one an unqualified query hits, so it is marked.
func TestCurrentSchemaIsMarkedInTheTree(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	if !h.waitForScreen(currentSchemaMarker) {
		t.Errorf("no schema is marked as current:\n%s", h.text())
	}
}
