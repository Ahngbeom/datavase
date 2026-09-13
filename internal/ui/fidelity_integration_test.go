//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
)

// What the server sent has to survive the whole way out: through the scan,
// the buffer, the grid it is read from and the file it is exported to. A
// failure here is the only kind this program cannot be forgiven — a value
// that is wrong but plausible is acted on, where a value that is missing is
// only investigated.
//
// The cases are the ones where a client can lose something quietly: a scale
// that trailing-zero arithmetic would round off, an unsigned integer past
// what a signed one holds, absence against emptiness, bytes that are not
// text, a timestamp with a fraction, and text that is neither ASCII nor one
// line.
func TestValuesSurviveFromTheServerToTheGridAndTheFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	const query = `SELECT
		CAST('12345.6700' AS DECIMAL(20,4))        AS scale_kept,
		CAST(18446744073709551615 AS UNSIGNED)     AS big_unsigned,
		NULL                                       AS absent,
		''                                         AS blank,
		CAST('2026-09-14 01:02:03.456789' AS DATETIME(6)) AS with_fraction,
		'한글 🌏'                                   AS not_ascii,
		'one
two'                                          AS two_lines`

	h.typeSQL(query)
	h.do(keymap.ActionRun)
	h.waitFor("the row", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 1 })

	// The file is what someone sends on, so it is checked against the value
	// the server holds rather than against whatever the screen had room for.
	var csv string
	h.inspect(func(a *App) bool {
		text, err := resultText(a.buf, a.content, formatCSV)
		if err != nil {
			t.Errorf("exporting: %v", err)
		}
		csv = text
		return true
	})

	for _, want := range []string{
		"12345.6700",           // not 12345.67
		"18446744073709551615", // not -1, and not 1.8446744073709552e+19
		"2026-09-14 01:02:03.456789",
		"한글 🌏",
		`"one` + "\n" + `two"`, // the newline is quoted, not dropped
	} {
		if !strings.Contains(csv, want) {
			t.Errorf("the exported file does not carry %q:\n%s", want, csv)
		}
	}

	// Absence and emptiness are two answers. CSV has no room to say which is
	// which — an empty field is both — so the distinction has to survive
	// where there is room for it: on screen, and in JSON.
	var js string
	h.inspect(func(a *App) bool {
		text, err := resultText(a.buf, a.content, formatJSON)
		if err != nil {
			t.Errorf("exporting: %v", err)
		}
		js = text
		return true
	})
	if !strings.Contains(js, `"absent": null`) || !strings.Contains(js, `"blank": ""`) {
		t.Errorf("JSON does not tell absence from emptiness:\n%s", js)
	}
	if got := h.gridColumn(2); len(got) != 1 || got[0] != result.NullText {
		t.Errorf("the grid shows %q for NULL, want %q", got, result.NullText)
	}
	if got := h.gridColumn(3); len(got) != 1 || got[0] != "" {
		t.Errorf("the grid shows %q for the empty string, want it empty", got)
	}

	// And the same values on screen, which is what gets read aloud in a
	// channel long before anyone exports anything.
	for column, want := range map[int]string{
		0: "12345.6700",
		1: "18446744073709551615",
		4: "2026-09-14 01:02:03.456789",
		5: "한글 🌏",
	} {
		if got := h.gridColumn(column); len(got) != 1 || got[0] != want {
			t.Errorf("the grid shows %q in column %d, want %q", got, column, want)
		}
	}
}
