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
