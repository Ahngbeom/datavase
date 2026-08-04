package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/match"
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
func (a *App) showSessions() { a.refreshSessions("") }

// refreshSessions reads the list again, keeping a message that matters more
// than the summary would.
//
// Refreshing after a kill is the obvious kindness and it takes away the one
// thing the user wanted confirmed: they stopped somebody's statement, and the
// status bar answers "3 connections · 1 working".
func (a *App) refreshSessions(keep string) {
	if keep == "" {
		a.notice("reading the process list…")
	}

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
			if keep != "" {
				a.notice(keep)
				return
			}
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

// killPhrase is the word that has to be typed before a kill goes through, or
// "" when buttons are enough.
//
// Stopping somebody else's work is the operation that most wants a
// confirmation nobody can wave through, and against production it costs the
// same deliberateness an unbounded DELETE does. Elsewhere the mistake is
// recoverable — the client reruns its query — and demanding a word for every
// one of them is how a demand stops being read.
func killPhrase(env config.Env, stop procs.Stop) string {
	if env != config.EnvProd {
		return ""
	}
	if stop == procs.StopConnection {
		// The connection goes and any transaction it held is rolled back, so
		// this is somebody's work rather than somebody's wait.
		return "KILL CONNECTION"
	}
	return "KILL"
}

// showKillSession offers the connections that can be stopped.
//
// A picker rather than a selection in the sessions tab, which is how every
// other choice here is made — and it sidesteps the failure a live list has:
// the list cannot reload under the cursor, because it is built when it opens
// and thrown away when it closes.
func (a *App) showKillSession(stop procs.Stop) {
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
			a.offerKill(listing, stop)
		})
	}()
}

func (a *App) offerKill(listing procs.Listing, stop procs.Stop) {
	killable := make([]procs.Process, 0, len(listing.Processes))
	for _, p := range listing.Processes {
		// Our own are not offered at all. procs.Kill refuses them too, but a
		// row you can select and cannot act on is worse than one that is not
		// there.
		if !a.conn.Owns(p.ID) {
			killable = append(killable, p)
		}
	}

	if len(killable) == 0 {
		a.notice(sessionsSummary(listing) + " — nothing here but this session")
		return
	}

	title := fmt.Sprintf(" stop a %s ", stop)
	box := a.newSearchBox("connection: ", title, pageKill, func(term string) []searchItem {
		return killChoices(killable, term, func(p procs.Process) {
			a.closeSearchBox(pageKill)
			a.confirmKill(p, stop)
		})
	})
	a.pages.AddPage(pageKill, centred(box, 84, 22), true, true)
}

// killChoices filters the connections, keeping the order they arrived in so
// that the longest-running is offered first.
func killChoices(ps []procs.Process, term string, choose func(procs.Process)) []searchItem {
	rows := make([]ranked, 0, len(ps))
	for _, p := range ps {
		proc := p

		// The id, the user and the statement are all things someone might
		// have in hand when they come here.
		haystack := fmt.Sprintf("%d %s %s %s", proc.ID, proc.User, proc.DB, collapse(proc.SQL))
		score, ok := match.Fuzzy(term, haystack)
		if !ok {
			continue
		}

		rows = append(rows, ranked{
			item: searchItem{
				primary:   fmt.Sprintf("%d  %s@%s  %s", proc.ID, proc.User, proc.Host, proc.Elapsed),
				secondary: collapse(proc.SQL),
				accept:    func() { choose(proc) },
			},
			score: score,
		})
	}

	items := sortRanked(rows)
	if len(items) == 0 {
		return []searchItem{message("no matching connection", "type an id, a user or part of a statement")}
	}
	return items
}

// confirmKill asks before stopping somebody else's work.
func (a *App) confirmKill(p procs.Process, stop procs.Stop) {
	what := fmt.Sprintf("Stop the %s on connection %d?\n\n%s@%s\n\n%s",
		stop, p.ID, p.User, p.Host, preview(p.SQL))

	if phrase := killPhrase(a.conn.DataSource().Env, stop); phrase != "" {
		a.typeToConfirm(what, phrase, "Stop", func() { a.kill(p, stop) }, nil)
		return
	}
	a.confirmDiscard(what, "Stop", func() { a.kill(p, stop) })
}

func (a *App) kill(p procs.Process, stop procs.Stop) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sessionsTimeout)
		defer cancel()

		err := procs.Kill(ctx, a.conn, p.ID, stop)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("%v", err))
				return
			}
			a.refreshSessions(fmt.Sprintf("stopped the %s on connection %d", stop, p.ID))
		})
	}()
}

// lockTreeText draws who is waiting on whom.
//
// A server that keeps its lock waits somewhere this does not look says so.
// An empty tree and a server that will not answer look identical on screen,
// and only one of them means nothing is stuck.
func lockTreeText(tree procs.LockTree, width int) string {
	if !tree.Supported {
		return "This server does not expose InnoDB lock waits where this looks" +
			" (information_schema.INNODB_LOCK_WAITS, performance_schema.data_lock_waits)."
	}
	if len(tree.Roots) == 0 {
		return "Nothing is waiting on a lock."
	}

	var b strings.Builder
	for i, root := range tree.Roots {
		if i > 0 {
			b.WriteString("\n")
		}
		writeBlocked(&b, root, "", "", width)
	}
	return b.String()
}

func writeBlocked(b *strings.Builder, n *procs.Blocked, prefix, childPrefix string, width int) {
	head := fmt.Sprintf("%d  %s@%s", n.Thread.ID, n.Thread.User, n.Thread.Host)
	switch {
	case n.Waited > 0:
		head += fmt.Sprintf("  [red]waiting %s[-]", n.Waited.Truncate(time.Second))
	case n.Thread.Idle:
		// The most common answer to "why is everything stuck", and the one a
		// list of running statements cannot give: it holds the lock and is
		// running nothing.
		head += "  [red]holding, idle in transaction[-]"
	default:
		head += "  [red]holding[-]"
	}

	fmt.Fprintf(b, "%s%s\n", prefix, result.EscapeTags(head))

	// A blocker's current statement is usually not the one that took the lock:
	// a transaction keeps what it took until it ends, whatever it does after.
	if n.Thread.SQL != "" {
		for _, line := range wrapPlain(result.EscapeTags(collapse(n.Thread.SQL)), width-len(childPrefix)-4) {
			fmt.Fprintf(b, "%s    %s\n", childPrefix, line)
		}
	}

	for i, w := range n.Waiters {
		elbow, spine := "├─ ", "│  "
		if i == len(n.Waiters)-1 {
			elbow, spine = "└─ ", "   "
		}
		writeBlocked(b, w, childPrefix+elbow, childPrefix+spine, width)
	}
}

// showLocks reads the lock waits and shows them.
func (a *App) showLocks() {
	a.notice("reading the lock waits…")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sessionsTimeout)
		defer cancel()

		tree, err := procs.LockWaits(ctx, a.conn)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("reading the lock waits: %v", err))
				return
			}

			a.sessionsView.SetText(lockTreeText(tree, a.paneWidth(a.sessionsView))).ScrollToBeginning()
			a.resultTabs.show(tabSessions)
			a.notice(lockSummary(tree))
		})
	}()
}

// lockSummary says what was found, keeping "nothing is waiting" and "this
// server will not say" apart.
func lockSummary(tree procs.LockTree) string {
	if !tree.Supported {
		return "this server does not expose InnoDB lock waits"
	}
	if len(tree.Roots) == 0 {
		return "nothing is waiting on a lock"
	}

	var waiting int
	for _, root := range tree.Roots {
		waiting += countWaiters(root)
	}
	return fmt.Sprintf("%s blocked by %s",
		plural(waiting, "connection"), plural(len(tree.Roots), "connection"))
}

func countWaiters(n *procs.Blocked) int {
	total := len(n.Waiters)
	for _, w := range n.Waiters {
		total += countWaiters(w)
	}
	return total
}
