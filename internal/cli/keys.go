package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/vim"
)

// keys prints the key map, or configuration for making a terminal deliver it.
func (a *App) keys(args []string) int {
	fs := flag.NewFlagSet("keys", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	ghostty := fs.Bool("ghostty", false, "print Ghostty configuration for the ⌘ bindings")
	iterm2 := fs.Bool("iterm2", false, "explain the iTerm2 settings")
	tmux := fs.Bool("tmux", false, "print tmux settings for modified keys")
	debug := fs.Bool("debug", false, "report what this terminal actually sends for each key press")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	km, err := a.keymap()
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	switch {
	case *ghostty:
		fmt.Fprint(a.Out, keymap.GhosttySnippet(km))
	case *iterm2:
		fmt.Fprint(a.Out, keymap.ITerm2Advice())
	case *tmux:
		fmt.Fprint(a.Out, keymap.TmuxSnippet())
	case *debug:
		return a.keyDebug()
	default:
		a.printKeys(km)
	}
	return exitOK
}

// keymap builds the effective key map: the configured preset with the user's
// overrides. `dv keys` has to show the same keyboard the interface uses, so
// both go through the same builder.
func (a *App) keymap() (*keymap.Map, error) {
	if a.Config == nil {
		return keymap.Default(), nil
	}

	km, err := keymap.FromConfig(a.Config.Keymap.Preset, a.Config.Keymap.Actions)
	if err != nil {
		return nil, fmt.Errorf("keymap: %w", err)
	}
	return km, nil
}

// printVimKeys lists the modal commands, which are the state machine's rather
// than the keymap's and so are not in the table above.
func printVimKeys(out io.Writer, km *keymap.Map) {
	if !km.Modal() {
		return
	}

	for _, group := range vim.Reference() {
		fmt.Fprintf(out, "\n%s\n", group.Title)
		for _, entry := range group.Entries {
			fmt.Fprintf(out, "%s  %s\n",
				keymap.PadLabel(entry.Keys, keyColumn), entry.Description)
		}
	}
}

// keyColumn is the width of the key column. Wide enough for "⌘⇧↩  ^⇧↩  ⇧F5".
const keyColumn = 22

// familiarWidth bounds the packed block's content, at the conventional
// terminal's width rather than a measured one. The two subtracted are the
// indent printKeys adds: without them a line reaches 82 cells and wraps
// there, and a wrapped line costs two rows — undoing the packing it wrapped.
const familiarWidth = 80 - 2

func (a *App) printKeys(km *keymap.Map) {
	mac := runtime.GOOS == "darwin"
	term := os.Getenv("TERM")

	ours, known := keymap.SplitByFamiliarity(keymap.AllActions())

	fmt.Fprintf(a.Out, "The %d that are dv's own\n", len(ours))
	for _, action := range ours {
		labels := make([]string, 0, 3)
		for _, b := range km.DisplayBindings(action) {
			labels = append(labels, b.Label(mac))
		}

		note := ""
		if action.Reserved() {
			note = "  (not built yet)"
		}
		if action == keymap.ActionCommandPalette {
			note += "  ← this one reaches the rest"
		}
		// Padded by display width rather than rune count: ⌘ and ⇥ take more
		// than one cell, and %-Ns would leave the column ragged.
		fmt.Fprintf(a.Out, "  %s  %s%s\n",
			keymap.PadLabel(strings.Join(labels, "  "), keyColumn),
			action.Describe(), note)
	}

	fmt.Fprintf(a.Out, "\nAlready what you expect — %d keys, nothing new\n", len(known))
	for _, line := range keymap.PackFamiliar(km, known, mac, familiarWidth) {
		fmt.Fprintf(a.Out, "  %s\n", line)
	}

	printVimKeys(a.Out, km)

	// The advice matters more than the table when the terminal cannot deliver
	// the primary bindings, so it goes last where it will be read.
	if advice := keymap.TerminalAdvice(term, km); advice != "" {
		fmt.Fprintf(a.Out, "\n%s\n", advice)
	}
	if !mac {
		return
	}
	fmt.Fprint(a.Out, "\n⌘ bindings need the terminal to forward them:\n"+
		"  dv keys --ghostty    # Ghostty\n"+
		"  dv keys --iterm2     # iTerm2\n")
}
