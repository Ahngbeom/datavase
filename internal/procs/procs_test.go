package procs

import (
	"testing"
	"time"
)

// Without the PROCESS privilege the server returns your own connections and no
// error, so a list of one means either "nothing else is running" or "you cannot
// see it" — opposite answers that look identical.
//
// The lines are shaped like the server's, with the password hash left out: the
// grant list is what is being read, and a real hash in a fixture is a secret in
// the repository for no gain.
func TestGrantsProcess(t *testing.T) {
	for _, tt := range []struct {
		name string
		line string
		want bool
	}{
		{
			name: "all privileges on the server",
			line: "GRANT ALL PRIVILEGES ON *.* TO `root`@`localhost` WITH GRANT OPTION",
			want: true,
		},
		{
			name: "PROCESS among others",
			line: "GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO `dba`@`%`",
			want: true,
		},
		{
			name: "PROCESS alone",
			line: "GRANT PROCESS ON *.* TO `watcher`@`%`",
			want: true,
		},
		{
			name: "lower case, as some servers print it",
			line: "grant process on *.* to `watcher`@`%`",
			want: true,
		},

		{
			name: "usage is no privilege at all",
			line: "GRANT USAGE ON *.* TO `dv_limited`@`%`",
			want: false,
		},
		{
			name: "everything on one schema is not everything on the server",
			line: "GRANT ALL PRIVILEGES ON `app`.* TO `dv_limited`@`%`",
			want: false,
		},
		{
			// The word can appear in a name, and a grant on a schema can
			// never confer PROCESS however it reads.
			name: "a schema that happens to be called process",
			line: "GRANT SELECT ON `process`.* TO `dv_limited`@`%`",
			want: false,
		},
		{
			// PROXY is not PROCESS, and it is granted ON something else
			// entirely.
			name: "a proxy grant",
			line: "GRANT PROXY ON ''@'%' TO 'root'@'localhost' WITH GRANT OPTION",
			want: false,
		},
		{
			name: "a user whose name contains the privilege",
			line: "GRANT SELECT ON *.* TO `process_reader`@`%`",
			want: false,
		},
		{"nothing at all", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := grantsProcess(tt.line); got != tt.want {
				t.Errorf("grantsProcess(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// A server in trouble has one statement running for minutes and forty
// connections asleep. Ordering by time alone buries it under whichever client
// has held a socket open the longest.
func TestOrderPutsWorkBeforeIdleness(t *testing.T) {
	ps := []Process{
		{ID: 1, Command: "Sleep", Elapsed: 3 * time.Hour},
		{ID: 2, Command: "Query", Elapsed: 4 * time.Second},
		{ID: 3, Command: "Sleep", Elapsed: 10 * time.Minute},
		{ID: 4, Command: "Query", Elapsed: 90 * time.Second},
	}

	Order(ps)

	var got []uint64
	for _, p := range ps {
		got = append(got, p.ID)
	}
	// The two queries first, longest first; then the sleepers, longest first.
	want := []uint64{4, 2, 1, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered %v, want %v", got, want)
		}
	}
}

func TestWorkingTellsAQueryFromAHeldSocket(t *testing.T) {
	for _, tt := range []struct {
		command string
		want    bool
	}{
		{"Query", true},
		{"Execute", true},
		{"Binlog Dump", true},
		{"Sleep", false},
		{"sleep", false},
		// A row with no command says nothing, and claiming it is working
		// would put it at the top of a list read for what is working.
		{"", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			if got := (Process{Command: tt.command}).Working(); got != tt.want {
				t.Errorf("Working(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// The server gives a flat list of pairs, which is what a DBA then correlates by
// hand. The thing worth finding is the connection at the bottom that waits on
// nothing, because that is the one to deal with.
func TestTreeFindsWhoIsAtTheBottom(t *testing.T) {
	// 30 waits on 20, which waits on 10. 10 waits on nobody.
	waits := []Wait{
		{Waiter: Thread{ID: 30}, Blocker: Thread{ID: 20}, Waited: 2 * time.Second},
		{Waiter: Thread{ID: 20}, Blocker: Thread{ID: 10}, Waited: 9 * time.Second},
	}

	roots := Tree(waits)
	if len(roots) != 1 {
		t.Fatalf("found %d roots, want the one nobody is blocking", len(roots))
	}
	if roots[0].Thread.ID != 10 {
		t.Errorf("the root is %d, want 10", roots[0].Thread.ID)
	}
	if roots[0].Waited != 0 {
		t.Errorf("the root reports waiting %v; it waits on nothing", roots[0].Waited)
	}

	if len(roots[0].Waiters) != 1 || roots[0].Waiters[0].Thread.ID != 20 {
		t.Fatalf("20 is not under 10: %+v", roots[0].Waiters)
	}
	twenty := roots[0].Waiters[0]
	if twenty.Waited != 9*time.Second {
		t.Errorf("20 waited %v, want 9s", twenty.Waited)
	}
	if len(twenty.Waiters) != 1 || twenty.Waiters[0].Thread.ID != 30 {
		t.Errorf("30 is not under 20: %+v", twenty.Waiters)
	}
}

// A blocker holding several locks appears once per lock, and one edge is
// enough — a thread listed three times under the same blocker reads as three
// connections.
func TestTreeDoesNotRepeatAPair(t *testing.T) {
	waits := []Wait{
		{Waiter: Thread{ID: 2}, Blocker: Thread{ID: 1}},
		{Waiter: Thread{ID: 2}, Blocker: Thread{ID: 1}},
		{Waiter: Thread{ID: 3}, Blocker: Thread{ID: 1}},
	}

	roots := Tree(waits)
	if len(roots) != 1 {
		t.Fatalf("found %d roots, want 1", len(roots))
	}
	if got := len(roots[0].Waiters); got != 2 {
		t.Errorf("the blocker has %d waiters, want 2", got)
	}
}

// A thread met first as a blocker carries no statement; met again as a waiter
// it does. Keeping the emptier description would lose the statement that is
// actually stuck.
func TestTreeKeepsTheFullerDescriptionOfAThread(t *testing.T) {
	waits := []Wait{
		{Waiter: Thread{ID: 2}, Blocker: Thread{ID: 1, Idle: true}},
		{Waiter: Thread{ID: 1, SQL: "UPDATE t SET n = 1"}, Blocker: Thread{ID: 9}},
	}

	roots := Tree(waits)
	var one *Blocked
	for _, r := range roots {
		for _, w := range r.Waiters {
			if w.Thread.ID == 1 {
				one = w
			}
		}
	}
	if one == nil {
		t.Fatalf("thread 1 is not in the tree: %+v", roots)
	}
	if one.Thread.SQL != "UPDATE t SET n = 1" {
		t.Errorf("thread 1 reads %q, want the statement it is running", one.Thread.SQL)
	}
}

// Nothing waiting is a tree with no roots, and it is not the same answer as a
// server that will not say.
func TestAnEmptyTreeIsNotAnUnsupportedOne(t *testing.T) {
	if got := Tree(nil); len(got) != 0 {
		t.Errorf("Tree(nil) = %v, want nothing", got)
	}
}
