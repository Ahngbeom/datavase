// Package sqlformat lays SQL out one clause per line.
//
// It changes only the whitespace between tokens and the case of reserved
// words. It does not know the grammar: it works from sqlparse's tokens, so a
// statement it does not recognise is reflowed conservatively rather than
// misread, and Format checks its own output against the input before
// returning it.
package sqlformat

import (
	"errors"
	"strings"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// ErrVersionComment is returned for SQL holding a /*! ... */ comment. The
// tokenizer unwraps those, so text rebuilt from its tokens would lose the
// markers and run version-gated code on every server.
var ErrVersionComment = errors.New("a /*! */ comment cannot be formatted without unwrapping it")

// errChanged means the self-check caught a layout that would alter the
// statement. It indicates a bug here, and refusing is the only safe answer.
// ErrSplitsToken is returned for a selection that starts or ends inside a
// token, or starts on a word that what precedes it makes a name. Read on its
// own, such a selection is formatted as something it is not.
var ErrSplitsToken = errors.New("the selection cuts through a name, string or comment")

var errChanged = errors.New("formatting would change the statement")

const indentUnit = "    "

// Format returns sql laid out by clause, with surrounding whitespace kept.
func Format(sql string) (string, error) {
	toks := sqlparse.Tokenize(sql)
	if hasVersionComment(sql, toks) {
		return "", ErrVersionComment
	}
	if len(toks) == 0 {
		return sql, nil
	}

	f := &formatter{src: sql, toks: toks, pending: noBreak}
	f.stack = []*frame{{clauseLevel: true}}
	body := f.run()

	out := sql[:toks[0].Pos] + body + sql[toks[len(toks)-1].End:]
	if !sameStatement(sql, out) {
		return "", errChanged
	}
	return out, nil
}

// FormatRange returns sql[from:to] laid out, refusing a selection that
// would not mean on its own what it means where it stands.
func FormatRange(sql string, from, to int) (string, error) {
	toks := sqlparse.Tokenize(sql)
	for i, tok := range toks {
		if within(from, tok) || within(to, tok) {
			return "", ErrSplitsToken
		}
		if tok.Pos == from && i > 0 && sqlparse.MakesAName(toks[i-1]) {
			return "", ErrSplitsToken
		}
	}
	return Format(sql[from:to])
}

func within(at int, tok sqlparse.Token) bool { return at > tok.Pos && at < tok.End }

// hasVersionComment reports whether sql holds a /*! the server would execute.
// The tokenizer unwraps those, leaving their markers outside every token;
// the same characters in a string or a comment lie inside one.
func hasVersionComment(sql string, toks []sqlparse.Token) bool {
	for at := 0; ; at += 3 {
		i := strings.Index(sql[at:], "/*!")
		if i < 0 {
			return false
		}
		at += i
		covered := false
		for _, tok := range toks {
			if at >= tok.Pos && at < tok.End {
				covered = true
				break
			}
		}
		if !covered {
			return true
		}
	}
}

// Offset carries a byte offset in before to the same character in after,
// where after differs from before only in whitespace and letter case.
func Offset(before, after string, at int) int {
	if at > len(before) {
		at = len(before)
	}
	n := 0
	for i := 0; i < at; i++ {
		if !isSpace(before[i]) {
			n++
		}
	}
	for i := 0; i < len(after); i++ {
		if isSpace(after[i]) {
			continue
		}
		if n == 0 {
			return i
		}
		n--
	}
	return len(after)
}

// frame is one level of parentheses. Only the statement itself and a
// subquery are clause levels; inside a function call or a column list,
// ORDER BY and AND belong to the call and must not break the line.
type frame struct {
	clauseLevel bool
	indent      int
	clause      string
	// between is set from BETWEEN until the AND that belongs to it, which
	// is part of the predicate rather than a new condition.
	between bool
	cases   int
}

const noBreak = -1

type formatter struct {
	src   string
	toks  []sqlparse.Token
	stack []*frame

	lines []string
	cur   strings.Builder

	// pending is the indent the next token starts a line at, or noBreak.
	pending int
	blank   bool
}

func (f *formatter) top() *frame { return f.stack[len(f.stack)-1] }

func (f *formatter) run() string {
	for i, tok := range f.toks {
		var prev, next *sqlparse.Token
		if i > 0 {
			prev = &f.toks[i-1]
		}
		if i+1 < len(f.toks) {
			next = &f.toks[i+1]
		}
		f.token(tok, prev, next)
	}
	f.lines = append(f.lines, strings.TrimRight(f.cur.String(), " "))
	return strings.Join(f.lines, "\n")
}

func (f *formatter) token(tok sqlparse.Token, prev, next *sqlparse.Token) {
	gap := ""
	if prev != nil {
		gap = f.src[prev.End:tok.Pos]
	}

	// A comment on the same line as what precedes it stays there, so a
	// trailing remark is not pushed under the next column.
	trailingComment := tok.Kind == sqlparse.Comment && !strings.Contains(gap, "\n")
	if f.pending != noBreak && !trailingComment {
		f.newline(f.pending, f.blank)
		f.pending, f.blank = noBreak, false
	}
	if tok.Kind == sqlparse.Comment && strings.Contains(gap, "\n") {
		f.newline(f.top().indent, false)
	}

	fr := f.top()
	word := tok.Kind == sqlparse.Word && !(prev != nil && sqlparse.MakesAName(*prev))
	upper := strings.ToUpper(tok.Text)
	text := tok.Text

	if word {
		if fr.clauseLevel && f.startsClause(upper, prev, next) {
			f.newline(fr.indent, false)
			fr.clause = upper
			if upper == "ON" {
				fr.clause = "ON DUPLICATE"
			}
			fr.between = false
		}
		switch upper {
		case "AND", "OR", "XOR":
			if fr.between && upper == "AND" {
				fr.between = false
			} else if fr.clauseLevel && conditionClause[fr.clause] {
				f.newline(fr.indent+1, false)
			}
		case "BETWEEN":
			fr.between = true
		case "ON":
			if fr.clauseLevel && fr.clause != "ON DUPLICATE" {
				fr.clause = "ON"
			}
		case "CASE":
			fr.cases++
		}

		switch {
		case sqlparse.IsReserved(upper):
			text = upper
		case upper == "END" && fr.cases > 0:
			fr.cases--
			text = upper
		case upper == "DUPLICATE" && prev != nil && strings.EqualFold(prev.Text, "ON"):
			text = upper
		}
	}

	if tok.Text == ")" && len(f.stack) > 1 {
		if fr.clauseLevel {
			f.newline(f.stack[len(f.stack)-2].indent, false)
		}
		f.stack = f.stack[:len(f.stack)-1]
	}

	if !f.atLineStart() && strings.ContainsAny(gap, " \t\r\n\f\v") {
		f.cur.WriteByte(' ')
	}
	f.cur.WriteString(text)

	switch {
	case tok.Text == "(":
		if next != nil && (next.IsKeyword("SELECT") || next.IsKeyword("WITH")) {
			f.stack = append(f.stack, &frame{clauseLevel: true, indent: fr.indent + 1})
			f.pending = fr.indent + 1
		} else {
			f.stack = append(f.stack, &frame{indent: fr.indent})
		}
	case tok.Text == ",":
		if fr.clauseLevel && fr.clause == "SELECT" {
			f.pending = fr.indent + 1
		}
	case tok.Text == ";":
		f.stack = f.stack[:1]
		*f.stack[0] = frame{clauseLevel: true}
		if next != nil {
			f.pending, f.blank = 0, true
		}
	case tok.Kind == sqlparse.Comment && !strings.HasPrefix(tok.Text, "/*"):
		if f.pending == noBreak {
			f.pending = f.top().indent
		}
	}
}

// startsClause reports whether a reserved word opens a new line.
func (f *formatter) startsClause(upper string, prev, next *sqlparse.Token) bool {
	prevUpper := ""
	if prev != nil && prev.Kind == sqlparse.Word {
		prevUpper = strings.ToUpper(prev.Text)
	}

	switch upper {
	case "SELECT", "FROM", "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "UNION":
		return true
	case "VALUES":
		// VALUES(col) inside ON DUPLICATE KEY UPDATE is a function.
		return prev == nil || (prev.Text != "=" && prev.Text != ",")
	case "SET":
		return prevUpper != "CHARACTER" && prevUpper != "CHARSET"
	case "ON":
		return next != nil && next.IsKeyword("DUPLICATE")
	case "LEFT", "RIGHT":
		// LEFT(str, n) is a function.
		if next != nil && next.Text == "(" {
			return false
		}
		return !joinWords[prevUpper]
	case "INNER", "CROSS", "NATURAL", "STRAIGHT_JOIN", "JOIN":
		return !joinWords[prevUpper]
	}
	return false
}

// newline ends the current line and starts one at the given indent. At the
// start of a line it only replaces the indent, so two rules asking for a
// break produce one.
func (f *formatter) newline(indent int, blank bool) {
	if !f.atLineStart() {
		f.lines = append(f.lines, strings.TrimRight(f.cur.String(), " "))
	}
	if blank && (len(f.lines) == 0 || f.lines[len(f.lines)-1] != "") {
		f.lines = append(f.lines, "")
	}
	f.cur.Reset()
	f.cur.WriteString(strings.Repeat(indentUnit, indent))
}

func (f *formatter) atLineStart() bool {
	return strings.TrimSpace(f.cur.String()) == ""
}

// sameStatement is the self-check: the output must hold the same tokens as
// the input, and nothing but whitespace and case may differ.
func sameStatement(before, after string) bool {
	if !strings.EqualFold(stripSpace(before), stripSpace(after)) {
		return false
	}
	a, b := sqlparse.Tokenize(before), sqlparse.Tokenize(after)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || !strings.EqualFold(a[i].Text, b[i].Text) {
			return false
		}
		if a[i].Kind != sqlparse.Word && a[i].Text != b[i].Text {
			return false
		}
	}
	return true
}

func stripSpace(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if !isSpace(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// conditionClause lists the clauses whose AND and OR each start a line.
var conditionClause = map[string]bool{"WHERE": true, "HAVING": true, "ON": true}

// joinWords can precede JOIN; only the first of them breaks the line.
var joinWords = map[string]bool{
	"LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true,
	"CROSS": true, "NATURAL": true,
}
