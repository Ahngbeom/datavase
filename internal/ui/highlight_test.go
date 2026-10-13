package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// roleAt reports the role highlights gives the byte at the start of word.
func roleAt(t *testing.T, sql, word string) role {
	t.Helper()
	at := strings.Index(sql, word)
	if at < 0 {
		t.Fatalf("%q is not in %q", word, sql)
	}
	for _, s := range highlights(sql) {
		if at >= s.from && at < s.to {
			return s.role
		}
	}
	return roleNone
}

func TestEachKindOfTokenGetsItsOwnRole(t *testing.T) {
	sql := "select name, 42 from `order` where note = 'from here' -- the from\n/* block */"
	tests := []struct {
		word string
		want role
	}{
		{word: "select", want: roleKeyword},
		{word: "name", want: roleNone},
		{word: "42", want: roleLiteral},
		{word: "`order`", want: roleNone},
		{word: "'from here'", want: roleLiteral},
		{word: "-- the from", want: roleComment},
		{word: "/* block */", want: roleComment},
	}
	for _, tt := range tests {
		if got := roleAt(t, sql, tt.word); got != tt.want {
			t.Errorf("%q has role %v, want %v", tt.word, got, tt.want)
		}
	}
}

// After a dot a reserved word is a column; colouring it as a keyword would
// make t.order read as an ORDER BY that is not there.
func TestAReservedWordAfterADotIsNotAKeyword(t *testing.T) {
	if got := roleAt(t, "select t.order", "order"); got != roleNone {
		t.Errorf("t.order has role %v, want none", got)
	}
}

// drawn renders text in a highlighted editor and returns the screen.
func drawn(t *testing.T, text string, width int, prepare func(*tview.TextArea)) tcell.SimulationScreen {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, 5)

	area := tview.NewTextArea().SetWrap(false)
	area.SetText(text, false)
	area.SetRect(0, 0, width, 5)
	editor := &highlightedEditor{TextArea: area}
	// TextArea resolves the caret and offsets on its first draw.
	editor.Draw(screen)
	if prepare != nil {
		prepare(area)
		editor.Draw(screen)
	}
	return screen
}

func styleAt(screen tcell.SimulationScreen, x, y int) (rune, tcell.Style) {
	r, _, style, _ := screen.GetContent(x, y)
	return r, style
}

func TestTheEditorDrawsKeywordsLiteralsAndCommentsApart(t *testing.T) {
	screen := drawn(t, "select 'a' -- c\nfrom t", 40, nil)

	cases := []struct {
		name   string
		x, y   int
		want   rune
		fg     tcell.Color
		bold   bool
		italic bool
	}{
		{name: "keyword", x: 0, y: 0, want: 's', fg: colourKeyword, bold: true},
		{name: "string", x: 7, y: 0, want: '\'', fg: colourLiteral},
		{name: "comment", x: 11, y: 0, want: '-', fg: colourComment, italic: true},
		{name: "keyword on the second line", x: 0, y: 1, want: 'f', fg: colourKeyword, bold: true},
		{name: "identifier", x: 5, y: 1, want: 't', fg: tcell.ColorDefault},
	}
	for _, c := range cases {
		r, style := styleAt(screen, c.x, c.y)
		fg, _, attrs := style.Decompose()
		if r != c.want {
			t.Errorf("%s: cell (%d,%d) holds %q, want %q", c.name, c.x, c.y, r, c.want)
		}
		if c.fg != tcell.ColorDefault && fg != c.fg {
			t.Errorf("%s: foreground %v, want %v", c.name, fg, c.fg)
		}
		if c.fg == tcell.ColorDefault && (fg == colourKeyword || fg == colourLiteral || fg == colourComment) {
			t.Errorf("%s: an identifier was coloured %v", c.name, fg)
		}
		if got := attrs&tcell.AttrBold != 0; got != c.bold {
			t.Errorf("%s: bold = %v, want %v", c.name, got, c.bold)
		}
		if got := attrs&tcell.AttrItalic != 0; got != c.italic {
			t.Errorf("%s: italic = %v, want %v", c.name, got, c.italic)
		}
	}
}

// A selection is drawn in the selected style; colouring over it would hide
// what is selected.
func TestASelectionIsNotPaintedOver(t *testing.T) {
	screen := drawn(t, "select 1", 40, func(a *tview.TextArea) { a.Select(0, 6) })

	_, style := styleAt(screen, 0, 0)
	if fg, _, _ := style.Decompose(); fg == colourKeyword {
		t.Error("a selected keyword was recoloured, hiding the selection")
	}
}

// Scrolled sideways, the colours must stay on the characters they belong to
// rather than on the screen columns they started in.
func TestColoursFollowTheTextWhenScrolledSideways(t *testing.T) {
	text := strings.Repeat("x", 30) + " from t"
	screen := drawn(t, text, 20, func(a *tview.TextArea) { a.SetOffset(0, 25) })

	if r, _ := styleAt(screen, 6, 0); r != 'f' {
		t.Fatalf("cell 6 holds %q; the editor did not scroll as the test assumes", r)
	}
	for x := 0; x < 20; x++ {
		r, style := styleAt(screen, x, 0)
		fg, _, _ := style.Decompose()
		if r == 'f' && fg != colourKeyword {
			t.Errorf("the f of FROM at column %d is %v, want the keyword colour", x, fg)
		}
		if r == 'x' && fg == colourKeyword {
			t.Errorf("an x at column %d took the keyword colour", x)
		}
	}
}

// A tab is several cells wide; columns after it must still line up.
func TestColoursLineUpAfterATab(t *testing.T) {
	screen := drawn(t, "\tfrom", 40, nil)

	for i, want := range "from" {
		x := tview.TabSize + i
		r, style := styleAt(screen, x, 0)
		if fg, _, _ := style.Decompose(); r != want || fg != colourKeyword {
			t.Errorf("cell %d holds %q in %v, want %q in the keyword colour", x, r, fg, want)
		}
	}
}

// After @ a word is a variable; colouring @where as a keyword would read as a
// WHERE clause that is not there.
func TestAVariableNamedLikeAKeywordIsNotAKeyword(t *testing.T) {
	if got := roleAt(t, "select @where", "where"); got != roleNone {
		t.Errorf("@where has role %v, want none", got)
	}
}
