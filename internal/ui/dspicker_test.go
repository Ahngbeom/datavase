package ui

import (
	"errors"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/rivo/tview"
)

// refusingStore wraps a Memory store and fails every Set, so a test can
// exercise what commit() does when the keychain refuses a write.
type refusingStore struct {
	*secret.Memory
	err error
}

func newRefusingStore(err error) *refusingStore {
	return &refusingStore{Memory: secret.NewMemory(), err: err}
}

func (r *refusingStore) Set(string, string) error { return r.err }

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

func TestPickerAddWithAPasswordStoresItUnderTheNewName(t *testing.T) {
	cfg := &config.Config{}
	secrets := secret.NewMemory()
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	candidate := config.DataSource{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}
	if err := p.commit("", candidate, "hunter2"); err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	if pw, err := secrets.Get("a"); err != nil || pw != "hunter2" {
		t.Errorf("secrets.Get(%q) = %q, %v, want \"hunter2\", nil", "a", pw, err)
	}
}

func TestPickerEditWithABlankPasswordLeavesTheStoredPasswordUnchanged(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}}}
	secrets := secret.NewMemory()
	if err := secrets.Set("a", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	candidate := config.DataSource{Name: "a", Host: "h2", User: "u", Port: config.DefaultPort}
	if err := p.commit("a", candidate, ""); err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	if pw, err := secrets.Get("a"); err != nil || pw != "hunter2" {
		t.Errorf("secrets.Get(%q) = %q, %v, want the password untouched", "a", pw, err)
	}
}

func TestPickerRenameWithABlankPasswordMovesThePasswordToTheNewName(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}}}
	secrets := secret.NewMemory()
	if err := secrets.Set("a", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	candidate := config.DataSource{Name: "b", Host: "h", User: "u", Port: config.DefaultPort}
	if err := p.commit("a", candidate, ""); err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	if pw, err := secrets.Get("b"); err != nil || pw != "hunter2" {
		t.Errorf("secrets.Get(%q) = %q, %v, want the password moved to the new name", "b", pw, err)
	}
	if _, err := secrets.Get("a"); !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("secrets.Get(%q) error = %v, want %v — the old name should be gone", "a", err, secret.ErrNotFound)
	}
}

// TestPickerRenameKeepsTheOldPasswordWhenTheKeychainRefusesTheNewName pins
// down the ordering bug: the config file is renamed (commit already saved
// it) before the password is moved, so a keychain failure must not also
// lose the only copy of the password by deleting the old entry.
func TestPickerRenameKeepsTheOldPasswordWhenTheKeychainRefusesTheNewName(t *testing.T) {
	refused := errors.New("keychain refused")
	secrets := newRefusingStore(refused)
	if err := secrets.Memory.Set("a", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}}}
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	candidate := config.DataSource{Name: "b", Host: "h", User: "u", Port: config.DefaultPort}
	err := p.commit("a", candidate, "")
	if err == nil {
		t.Fatal("commit() = nil, want an error reporting the stranded password")
	}
	var partial *savedWithoutPassword
	if !errors.As(err, &partial) {
		t.Errorf("commit() error = %v, want a *savedWithoutPassword", err)
	}
	if pw, getErr := secrets.Memory.Get("a"); getErr != nil || pw != "hunter2" {
		t.Errorf("secrets.Get(%q) = %q, %v, want the old entry kept", "a", pw, getErr)
	}
	if cfg.DataSources[0].Name != "b" {
		t.Errorf("DataSources[0].Name = %q, want the rename in the file to have gone through", cfg.DataSources[0].Name)
	}
}

func TestPickerRemoveDeletesTheKeychainEntryWithTheFileEntry(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u"}}}
	secrets := secret.NewMemory()
	if err := secrets.Set("a", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	p.remove("a")
	if _, err := secrets.Get("a"); !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("secrets.Get(%q) error = %v, want %v — the keychain entry should go with the file entry", "a", err, secret.ErrNotFound)
	}
}

func TestPickerCommitWithARefusingStoreReturnsSavedWithoutPassword(t *testing.T) {
	cfg := &config.Config{}
	secrets := newRefusingStore(errors.New("keychain refused"))
	p := newDSPicker(tview.NewApplication(), pickerDeps{cfg: cfg, save: func() error { return nil }, secrets: secrets})

	candidate := config.DataSource{Name: "a", Host: "h", User: "u", Port: config.DefaultPort}
	err := p.commit("", candidate, "hunter2")
	var partial *savedWithoutPassword
	if !errors.As(err, &partial) {
		t.Fatalf("commit() error = %v, want a *savedWithoutPassword", err)
	}
	if len(cfg.DataSources) != 1 || cfg.DataSources[0].Name != "a" {
		t.Errorf("DataSources = %+v, want the entry saved despite the keychain failure", cfg.DataSources)
	}
}

func TestPickerEscapeDuringAConnectDoesNotCloseTheLauncher(t *testing.T) {
	cfg := &config.Config{}
	closed := 0
	p := newDSPicker(tview.NewApplication(), pickerDeps{
		cfg: cfg, save: func() error { return nil },
		close: func() { closed++ },
	})

	p.setBusy(true)
	p.escape()
	if closed != 0 {
		t.Fatalf("close called %d times while busy, want 0 — the in-flight connect's session would leak", closed)
	}

	p.setBusy(false)
	p.escape()
	if closed != 1 {
		t.Errorf("close called %d times once idle, want 1", closed)
	}
}
