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
		{name: "second command"},
		{name: "use schema"},
	}
}

func TestTheCommandLineResolvesWhatWasTyped(t *testing.T) {
	tests := []struct {
		name string
		line string
		want cmdResolution
	}{
		// vim's own quit commands come first, and are matched whole. A user
		// who types these is not choosing from a list; they are reaching.
		{"q quits", "q", cmdResolution{intent: cmdQuit}},
		{"q! quits without asking", "q!", cmdResolution{intent: cmdForceQuit}},

		// Surrounding space is what a command line collects, not something the
		// user meant.
		{"space around the verb is ignored", "  q!  ", cmdResolution{intent: cmdForceQuit}},
		{"an empty line does nothing", "   ", cmdResolution{intent: cmdNothing}},

		// A palette name, spelled out.
		{"a full palette name runs it", "history", cmdResolution{intent: cmdPalette, name: "history"}},
		{"names with spaces work", "second command", cmdResolution{intent: cmdPalette, name: "second command"}},

		// An abbreviation is allowed only while it names one command.
		{"an unambiguous prefix runs it", "hist", cmdResolution{intent: cmdPalette, name: "history"}},
		{
			// cancel and commit are opposite outcomes, and "c" is one
			// keystroke away from either. Guessing here would pick one
			// silently instead of running the one actually meant.
			name: "an ambiguous prefix refuses and says what it could have meant",
			line: "c",
			want: cmdResolution{intent: cmdAmbiguous, among: []string{"cancel", "comment", "commit"}},
		},
		{"nothing matching is refused", "zzz", cmdResolution{intent: cmdUnknown}},
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
