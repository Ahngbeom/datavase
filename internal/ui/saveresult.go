package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// defaultResultPath is the file name offered before anyone types one: the
// datasource and the moment, so two saves in a row never collide and a
// directory of them still says where each came from.
func defaultResultPath(dsName string, at time.Time) string {
	return dsName + "-" + at.Format("20060102-150405") + ".csv"
}

// writeResultFile creates path with text in it, and refuses a path that is
// already a file.
//
// Refusing rather than replacing: the offered name carries the second, so
// a collision is never two saves of the same result — it is a typed path
// that already meant something to someone.
func writeResultFile(path, text string) error {
	path = expandHome(path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// The reason before the path, for the same reason the way back
			// goes before the driver's words: a path is long, the status bar
			// truncates from the right, and the reason is the half that says
			// whether to pick another name.
			return fmt.Errorf("already exists, not replaced: %s", path)
		}
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// expandHome turns a leading ~ into the home directory. The shell would
// have done it, but this path comes from a text field.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// saveSummary is the notice after a save. The path is repeated because a
// relative one resolved against a working directory the user may not have
// in mind.
func saveSummary(rows int, path string, size int, truncated bool) string {
	s := fmt.Sprintf("%s written to %s (%s)", plural(rows, "row"), path, humanBytes(size))
	if truncated {
		s += " · truncated at buffer_max"
	}
	return s
}

const pageSavePath = "save-path"

// promptSavePath asks where the CSV goes, offering a name that will not
// collide, and writes on Enter.
func (a *App) promptSavePath() {
	// A placeholder rather than text in the field. A field arriving with a
	// name already in it puts the caret after that name, so the first thing
	// anyone types extends it — and two names concatenate into a filename
	// that is perfectly valid and nobody chose. Empty, the suggestion is
	// still on screen and Enter still takes it.
	suggested := defaultResultPath(a.conn.DataSource().Name, time.Now())
	input := tview.NewInputField().SetLabel("save to: ").SetPlaceholder(suggested)
	input.SetBorder(true).SetTitle(" the result as a CSV file · ↩ for the name shown ")

	closePrompt := func() {
		a.pages.RemovePage(pageSavePath)
		a.app.SetFocus(a.grid)
	}
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			closePrompt()
			return
		}
		closePrompt()
		a.saveResult(chosenPath(input.GetText(), suggested))
	})

	a.pages.AddPage(pageSavePath, centred(input, 64, 3), true, true)
	a.app.SetFocus(input)
}

// saveResult writes the whole buffer, in the order the grid shows it, to path.
func (a *App) saveResult(path string) {
	text, err := resultText(a.buf, a.content, formatCSV)
	if err != nil {
		a.notice("save failed: " + err.Error())
		return
	}
	if err := writeResultFile(path, text); err != nil {
		a.notice("save failed: " + err.Error())
		return
	}
	a.notice(saveSummary(a.buf.RowCount(), path, len(text), a.buf.AtCapacity()))
}

// chosenPath is what to write to: what was typed, or the name the prompt
// suggested when nothing was.
func chosenPath(typed, suggested string) string {
	if p := strings.TrimSpace(typed); p != "" {
		return p
	}
	return suggested
}
