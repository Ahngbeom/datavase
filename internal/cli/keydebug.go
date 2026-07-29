package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// keyDebug reports what the terminal actually sends for each key press.
//
// When a binding "does not work" the cause is usually outside datavase — the
// terminal never sent the combination, or tmux swallowed it. Nothing else
// tells those apart from an application bug, so this prints the raw event
// alongside the action it resolves to.
func (a *App) keyDebug() int {
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(a.Err, "opening the terminal: %v\n", err)
		return exitError
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(a.Err, "initialising the terminal: %v\n", err)
		return exitError
	}

	km, err := a.keymap()
	if err != nil {
		screen.Fini()
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	lines := []string{
		"datavase key debug",
		"",
		fmt.Sprintf("TERM=%s   extended keys: %s",
			os.Getenv("TERM"), yesNo(keymap.SupportsExtendedKeys(""))),
		"",
		"Press keys to see what this terminal sends. Press Escape twice to exit.",
		"",
	}

	var (
		escapes int
		history []string
	)

	draw := func() {
		screen.Clear()
		row := 0
		for _, line := range lines {
			drawLine(screen, row, line)
			row++
		}
		// Newest first: the key just pressed is what the user is looking for.
		for i := len(history) - 1; i >= 0 && row < 40; i-- {
			drawLine(screen, row, history[i])
			row++
		}
		screen.Show()
	}
	draw()

	for {
		ev, ok := screen.PollEvent().(*tcell.EventKey)
		if !ok {
			draw()
			continue
		}

		if ev.Key() == tcell.KeyEscape {
			escapes++
			if escapes >= 2 {
				break
			}
		} else {
			escapes = 0
		}

		history = append(history, describeEvent(ev, km))
		if len(history) > 24 {
			history = history[len(history)-24:]
		}
		draw()
	}

	screen.Fini()
	return exitOK
}

// describeEvent renders one key press as the raw event plus its resolution.
func describeEvent(ev *tcell.EventKey, km *keymap.Map) string {
	var mods []string
	for _, m := range []struct {
		mask tcell.ModMask
		name string
	}{
		{tcell.ModCtrl, "Ctrl"},
		{tcell.ModMeta, "Super/Cmd"},
		{tcell.ModAlt, "Alt"},
		{tcell.ModShift, "Shift"},
	} {
		if ev.Modifiers()&m.mask != 0 {
			mods = append(mods, m.name)
		}
	}
	modText := "none"
	if len(mods) > 0 {
		modText = strings.Join(mods, "+")
	}

	keyText := tcell.KeyNames[ev.Key()]
	if ev.Key() == tcell.KeyRune {
		keyText = fmt.Sprintf("Rune %q (U+%04X)", ev.Rune(), ev.Rune())
	}

	action := km.Lookup(ev)
	actionText := "— not bound"
	if action != keymap.ActionNone {
		actionText = "→ " + action.String()
	}

	return fmt.Sprintf("%-34s mods: %-18s %s", keyText, modText, actionText)
}

func drawLine(screen tcell.Screen, row int, text string) {
	col := 0
	for _, r := range text {
		screen.SetContent(col, row, r, nil, tcell.StyleDefault)
		col++
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
