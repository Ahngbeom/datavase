package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/export"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/vim"
)

// command is one entry of the command palette.
type command struct {
	name    string
	summary string
	run     func(a *App)
}

// The two commands the guard's refusal has to be able to name. They are
// constants so the message and the palette entry cannot drift apart — a test
// checks the hint names a command the palette really offers.
const (
	cmdEnableWrites  = "write"
	cmdDisableWrites = "readonly"
)

// commands are what the palette offers. Keeping them in one list means the
// palette and any future ":" prompt cannot drift apart.
var commands = buildCommands()

func buildCommands() []command {
	cmds := []command{
		{
			name:    cmdEnableWrites,
			summary: "unlock writes to this production datasource for the session",
			run:     (*App).enableWrites,
		},
		{
			name:    cmdDisableWrites,
			summary: "lock writes again",
			run:     (*App).disableWrites,
		},
		{
			name:    "export csv",
			summary: "write the current result to a CSV file",
			run:     func(a *App) { a.exportResult(formatCSV) },
		},
		{
			name:    "export json",
			summary: "write the current result to a JSON file",
			run:     func(a *App) { a.exportResult(formatJSON) },
		},
		{
			name:    "history",
			summary: "search previously run statements",
			run:     (*App).showHistory,
		},
		{
			name:    "use schema",
			summary: "choose the schema unqualified names resolve against",
			run:     (*App).showUseSchema,
		},
		{
			name:    "refresh schema",
			summary: "reload the schema tree and completion cache",
			run:     (*App).loadSchemas,
		},
		{
			name:    "help",
			summary: "show the key reference",
			run:     (*App).showHelp,
		},
		{
			name:    "quit",
			summary: "leave datavase",
			run:     (*App).quit,
		},
	}

	// One entry per preset, generated rather than listed: a preset that the
	// palette does not offer is one nobody can reach without editing a file.
	for _, p := range keymap.Presets() {
		preset := p
		cmds = append(cmds, command{
			name:    "keymap " + string(preset),
			summary: "switch to the " + string(preset) + " keyboard for this session",
			run:     func(a *App) { a.setPreset(preset) },
		})
	}
	return cmds
}

// setPreset swaps the keyboard mid-session.
//
// Every lookup goes through a.keys at the moment a key arrives, so replacing
// the map is the whole switch — nothing needs rebinding. The user's own
// overrides are re-applied on top, since they were an opinion about keys, not
// about which keyboard those keys sat on.
func (a *App) setPreset(p keymap.Preset) {
	km, err := keymap.FromConfig(string(p), a.cfg.Keymap.Actions)
	if err != nil {
		a.notice(err.Error())
		return
	}

	a.keys = km
	// Switching keyboards mid-session starts the modal state over: being
	// dropped into insert mode by a keyboard change is not something anyone
	// would guess had happened.
	a.vim = vim.New()
	a.notice(fmt.Sprintf("keymap: %s — %s for keys", p, a.helpKeyLabel()))
}

// showCommandPalette offers every command, filtered as the user types.
//
// The filter is not decoration: the list outgrew a modest terminal once the
// keyboards were added to it, and a command you have to scroll blindly to
// find is one you will not find.
func (a *App) showCommandPalette() {
	box := a.newSearchBox("command: ", " commands ", pagePalette, func(term string) []searchItem {
		items := make([]searchItem, 0, len(commands))

		for _, c := range commands {
			cmd := c
			if !matchesCommand(cmd, term) {
				continue
			}
			items = append(items, searchItem{
				primary:   cmd.name,
				secondary: cmd.summary,
				accept: func() {
					a.closeSearchBox(pagePalette)
					cmd.run(a)
				},
			})
		}
		if len(items) == 0 {
			return []searchItem{message("no matching command", "press Escape to close")}
		}
		return items
	})

	a.pages.AddPage(pagePalette, centred(box, 66, 20), true, true)
}

// matchesCommand filters on the name and the summary, so "keyboard" finds the
// preset commands even though none of them says the word.
func matchesCommand(c command, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return true
	}
	return strings.Contains(strings.ToLower(c.name), term) ||
		strings.Contains(strings.ToLower(c.summary), term)
}

// enableWrites unlocks writes against production for this session only.
//
// It is never persisted: an unlock that outlived the session would quietly
// become the default, which is precisely the state the guard exists to
// prevent.
func (a *App) enableWrites() {
	a.status.writesEnabled = true
	a.notice("writes unlocked for this session — the status bar will keep saying so")
}

func (a *App) disableWrites() {
	a.status.writesEnabled = false
	a.notice("writes locked")
}

// exportFormat selects the writer.
type exportFormat int

const (
	formatCSV exportFormat = iota
	formatJSON
)

func (f exportFormat) extension() string {
	if f == formatJSON {
		return "json"
	}
	return "csv"
}

// exportResult writes the buffered result to a file.
//
// Only the rows actually held are written, and the name says as much when the
// result was truncated — a file that silently contains part of the answer is
// worse than one that admits it.
func (a *App) exportResult(format exportFormat) {
	if a.buf.ColumnCount() == 0 {
		a.notice("no result to export")
		return
	}

	path, err := a.exportPath(format)
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}

	columns := a.buf.Columns()
	rows := make([][]any, a.buf.RowCount())
	for i := range rows {
		rows[i] = a.buf.Row(i)
	}

	file, err := os.Create(path)
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}
	defer file.Close()

	if format == formatJSON {
		err = export.JSON(file, columns, rows)
	} else {
		err = export.CSV(file, columns, rows)
	}
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}

	message := fmt.Sprintf("wrote %d rows to %s", len(rows), path)
	if a.buf.AtCapacity() || a.status.truncated {
		message += " (truncated — the result was larger than the buffer)"
	}
	a.notice(message)
}

// exportPath builds a unique name in the working directory, so an export
// never silently overwrites an earlier one.
func (a *App) exportPath(format exportFormat) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	base := fmt.Sprintf("%s-%s.%s",
		sanitiseFilename(a.conn.DataSource().Name),
		time.Now().Format("20060102-150405"),
		format.extension())
	return filepath.Join(dir, base), nil
}

// sanitiseFilename keeps a datasource name usable as part of a filename.
func sanitiseFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "result"
	}
	return b.String()
}

// showHistory opens a searchable list of previously run statements.
func (a *App) showHistory() {
	if a.history == nil {
		a.notice("query history is unavailable")
		return
	}

	box := a.newSearchBox("search: ", " history ", pageHistory, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		entries, err := a.history.Search(ctx, term, 100)
		if err != nil {
			return []searchItem{message("search failed", err.Error())}
		}
		if len(entries) == 0 {
			return []searchItem{message("no matching statements", "")}
		}

		items := make([]searchItem, len(entries))
		for i, e := range entries {
			entry := e
			items[i] = searchItem{
				primary: oneLineSQL(entry.SQL),
				secondary: fmt.Sprintf("%s · %d rows · %s",
					entry.At.Local().Format("2006-01-02 15:04"), entry.Rows, entry.DataSource),
				accept: func() {
					a.closeSearchBox(pageHistory)
					a.editor.SetText(entry.SQL, true)
				},
			}
		}
		return items
	})

	a.pages.AddPage(pageHistory, centred(box, 90, 24), true, true)
}

// oneLineSQL flattens a statement so each history entry occupies one row.
func oneLineSQL(sql string) string {
	return result.Truncate(strings.Join(strings.Fields(sql), " "), 110)
}
