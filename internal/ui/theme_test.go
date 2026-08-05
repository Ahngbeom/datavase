package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// theme.go says each colour name here is a distinct role and gives it exactly
// one value. That only stays true while every caller goes through the names:
// a literal "[yellow]" is the same colour by coincidence rather than by
// intent, and it is what an edit to the role would leave behind.
//
// The failure this prevents is the one theme.go's own comment describes — six
// names for four values, and an environment cue indistinguishable from an
// error — reappearing one file over.
func TestNoWidgetNamesAColourInsteadOfARole(t *testing.T) {
	literal := regexp.MustCompile(`\[(aqua|yellow|red|gray|grey|green|blue|white|darkcyan|teal)[\]:]`)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		for i, line := range strings.Split(string(source), "\n") {
			// Comments are prose about colours, not colours.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if m := literal.FindString(line); m != "" {
				t.Errorf("%s:%d names a colour rather than a role: %s",
					file, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// A heading in the notice colour spends the one cue reserved for a state the
// user could forget they are in — an injected LIMIT, unlocked writes — on the
// word "Editing". Weight says "heading" without spending a hue, and survives
// a monochrome terminal, which is the same reason the active tab keeps its
// "▸" as well as its colour.
func TestSectionHeadingsDoNotSpendTheNoticeColour(t *testing.T) {
	for _, text := range []string{headingTag("Running"), headingTag("Files")} {
		if strings.Contains(text, colourNotice.String()) {
			t.Errorf("a section heading is drawn in the notice colour: %q", text)
		}
		if !strings.Contains(text, "b") {
			t.Errorf("a section heading carries no weight of its own: %q", text)
		}
	}
}
