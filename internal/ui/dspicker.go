package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// probeTimeout bounds the Test button, so a bastion that is not answering
// gives the form back.
const probeTimeout = 15 * time.Second

// pickerDeps is what the datasource list and form need from whoever hosts
// them — the launcher before a session exists, or the App during one.
type pickerDeps struct {
	cfg *config.Config
	// current is the name of the connected datasource, or empty.
	current string
	// save persists cfg. It is the host's because only the host knows the
	// path, and the App wants to say something after it succeeds.
	save func() error
	// secrets holds passwords. Nil means none can be stored, which the form
	// says rather than pretending.
	secrets secret.Store
	// probe answers "can this be reached", for the Test button.
	probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)
	// connect is what Enter on an entry does. The host owns the session.
	connect func(ds *config.DataSource)
	// close is Escape on the list.
	close func()
}

// dsPicker is the list of datasources and the form behind it.
type dsPicker struct {
	app  *tview.Application
	deps pickerDeps

	pages  *tview.Pages
	list   *tview.List
	status *tview.TextView
	busy   bool
}

const (
	pickerList = "list"
	pickerForm = "form"
)

func newDSPicker(app *tview.Application, deps pickerDeps) *dsPicker {
	p := &dsPicker{app: app, deps: deps, pages: tview.NewPages()}

	p.list = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	p.list.SetBorder(true).SetTitle(" datasources — Enter connect · a add · e edit · d delete · Esc close ")
	p.list.SetInputCapture(p.listKey)
	p.list.SetDoneFunc(p.escape)

	p.status = tview.NewTextView().SetDynamicColors(true)

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.list, 0, 1, true).
		AddItem(p.status, 1, 0, false)
	p.pages.AddPage(pickerList, body, true, true)

	p.renderList()
	return p
}

func (p *dsPicker) Primitive() tview.Primitive { return p.pages }

func (p *dsPicker) setStatus(text string) { p.status.SetText(text) }

// setBusy blocks a second connect while one is in flight.
func (p *dsPicker) setBusy(busy bool) { p.busy = busy }

func (p *dsPicker) renderList() {
	p.list.Clear()
	if len(p.deps.cfg.DataSources) == 0 {
		p.list.AddItem("no datasources yet", "press a to add one", 0, nil)
		return
	}
	for i := range p.deps.cfg.DataSources {
		ds := &p.deps.cfg.DataSources[i]
		detail := fmt.Sprintf("%s@%s:%d", ds.User, ds.Host, ds.Port)
		if ds.Tunnel != nil {
			detail += " via " + ds.Tunnel.Host
		}
		if ds.Name == p.deps.current {
			detail += " · connected"
		}
		p.list.AddItem(ds.Name, detail, 0, func() { p.connectTo(ds) })
	}
}

func (p *dsPicker) selected() *config.DataSource {
	i := p.list.GetCurrentItem()
	if i < 0 || i >= len(p.deps.cfg.DataSources) {
		return nil
	}
	return &p.deps.cfg.DataSources[i]
}

func (p *dsPicker) listKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() != tcell.KeyRune {
		return ev
	}
	switch ev.Rune() {
	case 'a':
		p.showForm(nil)
	case 'e':
		if ds := p.selected(); ds != nil {
			p.showForm(ds)
		}
	case 'd':
		if ds := p.selected(); ds != nil {
			p.confirmDelete(ds)
		}
	default:
		return ev
	}
	return nil
}

func (p *dsPicker) connectTo(ds *config.DataSource) {
	if p.busy {
		p.setStatus("still connecting…")
		return
	}
	if p.deps.connect != nil {
		p.deps.connect(ds)
	}
}

// escape is Enter's opposite on the list: it must not close over a connect
// in flight, or the session that connect eventually produces has nowhere to
// go and is never closed.
func (p *dsPicker) escape() {
	if p.busy {
		p.setStatus("still connecting…")
		return
	}
	if p.deps.close != nil {
		p.deps.close()
	}
}

// confirmDelete asks, then removes the entry and its stored password.
func (p *dsPicker) confirmDelete(ds *config.DataSource) {
	name := ds.Name
	modal := newModal().
		SetText(fmt.Sprintf("Delete %s?\n\nIts stored password goes with it.", name)).
		AddButtons([]string{"Cancel", "Delete"}).
		SetDoneFunc(func(_ int, label string) {
			p.pages.RemovePage("confirm")
			p.app.SetFocus(p.list)
			if label != "Delete" {
				return
			}
			p.remove(name)
		})
	p.pages.AddPage("confirm", modal, true, true)
	p.app.SetFocus(modal)
}

// remove filters name out of the list and saves. A failed save is rolled
// back to the snapshot taken before the filter, so the entry is not left
// looking deleted in memory when the file on disk still has it.
func (p *dsPicker) remove(name string) {
	before := append([]config.DataSource(nil), p.deps.cfg.DataSources...)
	kept := p.deps.cfg.DataSources[:0]
	for _, ds := range before {
		if ds.Name != name {
			kept = append(kept, ds)
		}
	}
	p.deps.cfg.DataSources = kept
	if err := p.deps.save(); err != nil {
		p.deps.cfg.DataSources = before
		p.setStatus(tag(colourDanger, "saving: "+err.Error()))
		return
	}
	if p.deps.secrets != nil {
		// A missing entry is not a failure: there may never have been one.
		_ = p.deps.secrets.Delete(name)
	}
	p.renderList()
	p.setStatus("deleted " + name)
}

// savedWithoutPassword reports that the configuration file was written but
// the keychain refused the password that goes with it, so the form must
// close on the strength of the save rather than stay open — retrying would
// re-run a commit that already succeeded and read to the user as a
// duplicate entry.
type savedWithoutPassword struct{ err error }

func (e *savedWithoutPassword) Error() string {
	return "saved, but the keychain refused the password: " + e.err.Error()
}

func (e *savedWithoutPassword) Unwrap() error { return e.err }

// commit validates candidate, applies it under editing (empty for a new
// entry) and saves. A failed save is rolled back to the snapshot taken
// before the mutation, so the in-memory list matches the file on disk and a
// retry under the same name is not refused as a duplicate of itself.
func (p *dsPicker) commit(editing string, candidate config.DataSource, password string) error {
	if err := validateDataSource(p.deps.cfg, editing, candidate); err != nil {
		return err
	}
	if password != "" && p.deps.secrets == nil {
		return errors.New("no keychain here; set DATAVASE_PASSWORD_<NAME> instead")
	}

	before := append([]config.DataSource(nil), p.deps.cfg.DataSources...)
	if editing == "" {
		p.deps.cfg.DataSources = append(p.deps.cfg.DataSources, candidate)
	} else {
		for i := range p.deps.cfg.DataSources {
			if p.deps.cfg.DataSources[i].Name == editing {
				p.deps.cfg.DataSources[i] = candidate
			}
		}
	}
	if err := p.deps.save(); err != nil {
		p.deps.cfg.DataSources = before
		return fmt.Errorf("saving: %w", err)
	}

	if password != "" {
		if err := p.deps.secrets.Set(candidate.Name, password); err != nil {
			return &savedWithoutPassword{err: err}
		}
	}
	if editing != "" && editing != candidate.Name && p.deps.secrets != nil {
		if password == "" {
			// A rename moves the password with the entry, if there is one.
			// The old entry is deleted only once the copy under the new name
			// has actually landed, so a keychain failure here leaves the
			// password retrievable under the name it is still filed as.
			if old, err := p.deps.secrets.Get(editing); err == nil {
				if err := p.deps.secrets.Set(candidate.Name, old); err != nil {
					return &savedWithoutPassword{err: fmt.Errorf("renamed, but the password could not be moved: %w", err)}
				}
			}
		}
		_ = p.deps.secrets.Delete(editing)
	}
	if editing != "" && editing == p.deps.current && candidate.Name != editing {
		// Otherwise the rename makes the list stop marking it as connected.
		p.deps.current = candidate.Name
	}
	return nil
}

// showForm opens the add form (ds nil) or the edit form.
func (p *dsPicker) showForm(ds *config.DataSource) {
	editing := ""
	fields := dsFields{port: "", tls: string(config.TLSPreferred)}
	if ds != nil {
		editing = ds.Name
		fields = fieldsFromDataSource(ds)
	}
	password := ""

	tlsModes := []string{string(config.TLSPreferred), string(config.TLSRequired),
		string(config.TLSVerifyCA), string(config.TLSVerifyIdentity), string(config.TLSDisabled)}
	tlsIndex := 0
	for i, m := range tlsModes {
		if m == fields.tls {
			tlsIndex = i
		}
	}

	form := tview.NewForm()
	form.AddInputField("name", fields.name, 30, nil, func(s string) { fields.name = s })
	form.AddInputField("host", fields.host, 30, nil, func(s string) { fields.host = s })
	form.AddInputField("port", fields.port, 6, nil, func(s string) { fields.port = s })
	form.AddInputField("user", fields.user, 30, nil, func(s string) { fields.user = s })
	passwordLabel := "password"
	if ds != nil {
		passwordLabel = "password (blank keeps the stored one)"
	}
	form.AddPasswordField(passwordLabel, "", 30, '•', func(s string) { password = s })
	form.AddInputField("database", fields.database, 30, nil, func(s string) { fields.database = s })
	form.AddDropDown("tls", tlsModes, tlsIndex, func(option string, _ int) { fields.tls = option })
	form.AddInputField("tls_ca (PEM file, verify modes only)", fields.tlsCA, 30, nil, func(s string) { fields.tlsCA = s })
	form.AddInputField("tunnel host (blank: no tunnel)", fields.tunnelHost, 30, nil, func(s string) { fields.tunnelHost = s })
	form.AddInputField("tunnel port", fields.tunnelPort, 6, nil, func(s string) { fields.tunnelPort = s })
	form.AddInputField("tunnel user", fields.tunnelUser, 30, nil, func(s string) { fields.tunnelUser = s })
	form.AddInputField("tunnel identity (key file)", fields.tunnelIdentity, 30, nil, func(s string) { fields.tunnelIdentity = s })

	message := tview.NewTextView().SetDynamicColors(true)
	say := func(text string) { message.SetText(text) }

	closeForm := func() {
		p.pages.RemovePage(pickerForm)
		p.app.SetFocus(p.list)
	}

	form.AddButton("Test", func() {
		candidate, err := fields.toDataSource()
		if err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		if err := validateDataSource(p.deps.cfg, editing, candidate); err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		pw := password
		if pw == "" && p.deps.secrets != nil && editing != "" {
			pw, _ = p.deps.secrets.Get(editing)
		}
		say("testing…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
			defer cancel()
			version, err := p.deps.probe(ctx, &candidate, pw)
			p.app.QueueUpdateDraw(func() {
				if err != nil {
					say(tag(colourDanger, err.Error()))
					return
				}
				say("reachable · " + version)
			})
		}()
	})

	form.AddButton("Save", func() {
		candidate, err := fields.toDataSource()
		if err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		if err := p.commit(editing, candidate, password); err != nil {
			var partial *savedWithoutPassword
			if errors.As(err, &partial) {
				p.renderList()
				p.setStatus(tag(colourDanger, err.Error()))
				closeForm()
				return
			}
			say(tag(colourDanger, err.Error()))
			return
		}
		p.renderList()
		p.setStatus("saved " + candidate.Name)
		closeForm()
	})
	form.AddButton("Cancel", closeForm)
	form.SetCancelFunc(closeForm)

	title := " add a datasource "
	if ds != nil {
		title = fmt.Sprintf(" edit %s ", ds.Name)
	}
	form.SetBorder(true).SetTitle(title)

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(message, 1, 0, false)
	p.pages.AddPage(pickerForm, body, true, true)
	p.app.SetFocus(form)
}
