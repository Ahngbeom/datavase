//go:build integration

package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

// The schema name on the top bar is where someone looks to see which schema
// an unqualified statement will reach, so it is where they reach to change
// it. Finding it inert sends them to the key reference.
func TestClickingTheSchemaOnTheTopBarOffersToChangeIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.clickZone(zoneSchema, -1)

	h.waitFor("the schema chooser to open", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageUseSchema
	})
}

// The help key is written on the top bar. A hint that names a key and does
// nothing when pressed on is worse than no hint.
func TestClickingTheHelpHintOpensTheKeyReference(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.clickZone(zoneHelp, -1)

	h.waitFor("the key reference to open", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageHelp
	})
}

// A misclick on the production marker must not be able to look like it
// changed the environment.
func TestTheEnvironmentChipAnswersNoClick(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	var before string
	h.inspect(func(a *App) bool {
		before, _ = a.pages.GetFrontPage()
		return true
	})

	h.click(1, 0)

	var after string
	h.inspect(func(a *App) bool {
		after, _ = a.pages.GetFrontPage()
		return true
	})
	if after != before {
		t.Errorf("clicking the environment chip opened %q", after)
	}
}
