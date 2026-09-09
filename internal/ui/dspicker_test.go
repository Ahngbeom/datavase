package ui

import (
	"errors"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/rivo/tview"
)

func newTestPicker(cfg *config.Config, save func() error) *dsPicker {
	return newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: save})
}

func TestPickerCommitRollsBackTheListWhenSaveFails(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u"}}}
	p := newTestPicker(cfg, func() error { return errors.New("disk full") })

	candidate := config.DataSource{Name: "b", Host: "h2", User: "u2", Port: config.DefaultPort}
	if err := p.commit("", candidate, ""); err == nil {
		t.Fatal("commit() = nil, want the save error")
	}
	if len(cfg.DataSources) != 1 {
		t.Errorf("DataSources = %+v, want the failed add rolled back", cfg.DataSources)
	}
}

func TestPickerCommitRetryAfterAFailedSaveIsNotRefusedAsADuplicate(t *testing.T) {
	cfg := &config.Config{}
	failing := errors.New("disk full")
	p := newTestPicker(cfg, func() error { return failing })

	candidate := config.DataSource{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}
	_ = p.commit("", candidate, "")
	err := p.commit("", candidate, "")
	if err == nil || !errors.Is(err, failing) {
		t.Errorf("second commit() error = %v, want the save error again, not a duplicate refusal", err)
	}
}

func TestPickerRemoveRollsBackTheListWhenSaveFails(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u"}, {Name: "b", Host: "h", User: "u"}}}
	p := newTestPicker(cfg, func() error { return errors.New("disk full") })

	p.remove("a")
	if len(cfg.DataSources) != 2 {
		t.Errorf("DataSources = %+v, want the failed delete rolled back", cfg.DataSources)
	}
}

func TestPickerCommitRenameKeepsTheConnectedMarkOnTheRenamedEntry(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "prod", Host: "h", User: "u"}}}
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, current: "prod", save: func() error { return nil }})

	renamed := config.DataSource{Name: "prod-renamed", Host: "h", User: "u", Port: config.DefaultPort}
	if err := p.commit("prod", renamed, ""); err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	if p.deps.current != "prod-renamed" {
		t.Errorf("deps.current = %q, want the renamed name", p.deps.current)
	}
}
