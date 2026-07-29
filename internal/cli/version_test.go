package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Every spelling has to work. Someone diagnosing a bug reaches for whichever
// one their fingers know, and being told "flag provided but not defined" is
// one more thing to work out before the actual problem.
func TestVersionIsAnsweredUnderEverySpelling(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer

			if !HandleVersion(&out, args) {
				t.Fatalf("HandleVersion(%v) = false, want it handled", args)
			}

			got := strings.TrimSpace(out.String())
			if !strings.HasPrefix(got, "dv ") {
				t.Errorf("printed %q, want it to name the program", got)
			}
			if len(strings.Fields(got)) < 2 {
				t.Errorf("printed %q, want a version after the name", got)
			}
		})
	}
}

// It must not swallow anything else, or `dv open -v-shaped-name` would print
// a version instead of connecting.
func TestHandleVersionIgnoresEverythingElse(t *testing.T) {
	for _, args := range [][]string{
		{}, {"open"}, {"ls"}, {"open", "-v"}, {"keys", "--version"}, {"-c", "x.yaml"},
	} {
		var out bytes.Buffer

		if HandleVersion(&out, args) {
			t.Errorf("HandleVersion(%v) = true, want it left alone", args)
		}
		if out.Len() > 0 {
			t.Errorf("HandleVersion(%v) printed %q, want nothing", args, out.String())
		}
	}
}

// The version belongs in the usage text too, since that is where someone
// looks when they do not already know the command exists.
func TestUsageMentionsTheVersionCommand(t *testing.T) {
	h := newHarness(t)
	h.app.Run([]string{"help"})

	if !strings.Contains(h.err.String(), "dv version") {
		t.Errorf("usage does not mention the version command:\n%s", h.err)
	}
}
