//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// Nothing else on screen says which schema an unqualified query will hit.
func TestStatusBarShowsTheCurrentSchema(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if !strings.Contains(h.text(), "@"+testmysql.DefaultDatabase) {
		t.Errorf("the status bar does not show the current schema:\n%s", h.text())
	}
}
