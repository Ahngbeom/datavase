package ui

import (
	"strings"
	"testing"
)

// A stand-in palette, so that these tests describe the resolution rules rather
// than the command list of the day. The real list gets its own test below,
// which is where a name that breaks a rule should show up.
func testCommands() []command {
	return []command{
		{name: "cancel"},
		{name: "commit"},
		{name: "comment"},
		{name: "history"},
		{name: "go to table"},
		{name: "use schema"},
		{name: "unlock writes", exact: true},
	}
}

func TestTheCommandLineResolvesWhatWasTyped(t *testing.T) {
	tests := []struct {
		name string
		line string
		want cmdResolution
	}{
		// vim's own file commands come first, and are matched whole. A user
		// who types these is not choosing from a list; they are reaching.
		{"w saves", "w", cmdResolution{intent: cmdSave}},
		{"write saves too", "write", cmdResolution{intent: cmdSave}},
		{"q quits", "q", cmdResolution{intent: cmdQuit}},
		{"q! quits without asking", "q!", cmdResolution{intent: cmdForceQuit}},
		{"wq saves then quits", "wq", cmdResolution{intent: cmdSaveQuit}},
		{"x is wq", "x", cmdResolution{intent: cmdSaveQuit}},
		{"e with no file opens the finder", "e", cmdResolution{intent: cmdEdit}},
		{"e takes a path", "e sql/001.sql", cmdResolution{intent: cmdEdit, arg: "sql/001.sql"}},

		// Surrounding space is what a command line collects, not something the
		// user meant.
		{"space around the verb is ignored", "  wq  ", cmdResolution{intent: cmdSaveQuit}},
		{"an empty line does nothing", "   ", cmdResolution{intent: cmdNothing}},

		// A palette name, spelled out.
		{"a full palette name runs it", "history", cmdResolution{intent: cmdPalette, name: "history"}},
		{"names with spaces work", "go to table", cmdResolution{intent: cmdPalette, name: "go to table"}},

		// An abbreviation is allowed only while it names one command.
		{"an unambiguous prefix runs it", "hist", cmdResolution{intent: cmdPalette, name: "history"}},
		{
			// cancel and commit are opposite outcomes, and "c" is one
			// keystroke away from either. Guessing here would be the same
			// class of mistake as a guard that lets through what it could not
			// classify.
			name: "an ambiguous prefix refuses and says what it could have meant",
			line: "c",
			want: cmdResolution{intent: cmdAmbiguous, among: []string{"cancel", "comment", "commit"}},
		},
		{"nothing matching is refused", "zzz", cmdResolution{intent: cmdUnknown}},

		// The unlock is reachable only by its whole name. A prefix that
		// happens to be unique today stops being unique the moment another
		// command is added, and this is the one command whose meaning must
		// never depend on what else is in the list.
		{"the unlock runs from its full name", "unlock writes", cmdResolution{intent: cmdPalette, name: "unlock writes"}},
		{"the unlock does not run from a prefix", "unlock", cmdResolution{intent: cmdUnknown}},
		{"nor from most of it", "unlock write", cmdResolution{intent: cmdUnknown}},
		{
			// Keeping the unlock out of the answer must not take it out of the
			// count. Dropping it from the candidates would leave "use schema"
			// alone under "u" and make it run — a command becoming easier to
			// reach because a dangerous one was hidden next to it.
			name: "a prefix the unlock shares stays ambiguous rather than skipping to the other one",
			line: "u",
			want: cmdResolution{intent: cmdAmbiguous, among: []string{"unlock writes", "use schema"}},
		},
		{"once past them both, the safe one runs", "use", cmdResolution{intent: cmdPalette, name: "use schema"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCommandLine(tt.line, testCommands())

			if got.intent != tt.want.intent || got.arg != tt.want.arg || got.name != tt.want.name {
				t.Errorf("resolveCommandLine(%q) = %+v, want %+v", tt.line, got, tt.want)
			}
			if strings.Join(got.among, ",") != strings.Join(tt.want.among, ",") {
				t.Errorf("resolveCommandLine(%q) could have meant %v, want %v", tt.line, got.among, tt.want.among)
			}
		})
	}
}

// The rules above are only worth anything against the list people actually
// type at.
func TestTheRealPaletteAnswersTheReflexes(t *testing.T) {
	cmds := paletteCommands()

	for line, want := range map[string]cmdIntent{
		"w":             cmdSave,
		"write":         cmdSave,
		"q":             cmdQuit,
		"q!":            cmdForceQuit,
		"wq":            cmdSaveQuit,
		"unlock writes": cmdPalette,
	} {
		if got := resolveCommandLine(line, cmds); got.intent != want {
			t.Errorf(":%s resolved to %v, want %v", line, got.intent, want)
		}
	}

	// The one that must not resolve at all. "u" is undo to a vim user, and
	// the only reason it is not already the unlock is that "use schema"
	// happens to share the letter — which is luck, not a rule.
	for _, line := range []string{"u", "un", "unlock", "unlock write"} {
		if got := resolveCommandLine(line, cmds); got.intent == cmdPalette && got.name == cmdEnableWrites {
			t.Errorf(":%s unlocked production writes; only the whole name may", line)
		}
	}
}
