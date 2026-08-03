package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/procs"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/rivo/tview"
)

// sessionsTimeout bounds the listing. It is two statements against
// information_schema, so anything slower is the server in trouble — which is
// usually why it was asked.
const sessionsTimeout = 20 * time.Second

const sessionsPlaceholder = "Nothing read yet."

func (a *App) buildSessionsTab() tview.Primitive {
	a.sessionsView = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	a.sessionsView.SetText(sessionsPlaceholder)
	return a.sessionsView
}

// showSessions reads what else is running and shows it.
//
// Reading is explicit rather than on a timer. A list that reloads under the
// cursor is how the wrong session gets acted on, and the moment this matters
// is the moment someone is about to act on a row.
func (a *App) showSessions() {
	a.notice("reading the process list…")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sessionsTimeout)
		defer cancel()

		listing, err := procs.List(ctx, a.conn)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("reading the process list: %v", err))
				return
			}

			a.sessionsView.SetText(sessionsText(listing, a.paneWidth(a.sessionsView))).ScrollToBeginning()
			a.resultTabs.show(tabSessions)
			a.notice(sessionsSummary(listing))
		})
	}()
}

// sessionsSummary says what was found, and admits what could not be seen.
//
// A user without the PROCESS privilege is shown their own connections and no
// error, so a list of one means either "nothing else is running" or "you cannot
// see it" — and those are opposite answers to the question that was asked.
func sessionsSummary(l procs.Listing) string {
	var working int
	for _, p := range l.Processes {
		if p.Working() {
			working++
		}
	}

	summary := fmt.Sprintf("%s · %d working", plural(len(l.Processes), "connection"), working)
	if !l.Complete {
		summary += " — only your own: this user has no PROCESS privilege"
	}
	return summary
}

// sessionsText lays the listing out for a pane of the given width.
//
// The statement goes on its own line under the connection rather than in a
// column, because it is the only field with no useful maximum width and the
// one worth reading in full.
func sessionsText(l procs.Listing, width int) string {
	if len(l.Processes) == 0 {
		return "No connections."
	}

	var b strings.Builder
	for i, p := range l.Processes {
		if i > 0 {
			b.WriteString("\n")
		}
		writeProcess(&b, p, width)
	}
	return b.String()
}

func writeProcess(b *strings.Builder, p procs.Process, width int) {
	head := fmt.Sprintf("%d  %s@%s", p.ID, p.User, p.Host)
	if p.DB != "" {
		head += "  " + p.DB
	}

	state := p.Command
	if p.State != "" {
		state += " · " + p.State
	}
	fmt.Fprintf(b, "%s\n  %s  %s\n",
		result.EscapeTags(head), colouredAge(p), result.EscapeTags(state))

	if p.SQL != "" {
		for _, line := range wrapPlain(result.EscapeTags(collapse(p.SQL)), width-4) {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}

// colouredAge shows how long the connection has been in this command, loud
// once it is long enough to be the answer to why something is slow.
func colouredAge(p procs.Process) string {
	age := p.Elapsed.Truncate(time.Second).String()
	if !p.Working() {
		return fmt.Sprintf("[gray]idle %s[-]", age)
	}
	if p.Elapsed >= longRunning {
		return fmt.Sprintf("[red]%s[-]", age)
	}
	return age
}

// longRunning is when an in-flight statement stops being ordinary. It is the
// same order as a human's patience rather than a server's, because the list is
// read by the human.
const longRunning = 10 * time.Second

// collapse puts a statement on one logical line so it wraps predictably.
func collapse(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

// wrapPlain breaks text to a width, without the tree indentation the plan
// needs.
func wrapPlain(text string, width int) []string {
	if width < 20 {
		width = 20
	}

	var (
		lines []string
		line  []rune
	)
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		if len(line) > 0 && len(line)+1+len(runes) > width {
			lines = append(lines, string(line))
			line = nil
		}
		if len(line) > 0 {
			line = append(line, ' ')
		}
		line = append(line, runes...)
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	return lines
}

// paneWidth is the room a pane has, read rather than assumed.
func (a *App) paneWidth(v *tview.TextView) int {
	_, _, width, _ := v.GetInnerRect()
	if width < 20 {
		return 20
	}
	return width
}
