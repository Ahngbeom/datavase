// Package sqlparse provides a lightweight MySQL tokenizer.
//
// It is not a full parser. Its job is to answer the questions guard and the
// editor ask — where does a statement start and end, what kind is it, does
// it have a top-level WHERE — with enough precision that literals, comments
// and quoted identifiers can never be mistaken for syntax.
package sqlparse

import "strings"

// Kind classifies a token.
type Kind int

const (
	// Word is a bare identifier or keyword.
	Word Kind = iota
	// Ident is a backtick-quoted identifier.
	Ident
	// String is a quoted literal.
	String
	// Number is a numeric literal.
	Number
	// Punct is an operator, separator or parenthesis.
	Punct
	// Comment is text the server ignores.
	Comment
)

func (k Kind) String() string {
	switch k {
	case Word:
		return "word"
	case Ident:
		return "ident"
	case String:
		return "string"
	case Number:
		return "number"
	case Punct:
		return "punct"
	case Comment:
		return "comment"
	default:
		return "unknown"
	}
}

// Token is a single lexical element with its source span.
type Token struct {
	Kind Kind
	Text string
	// Pos and End delimit the token in the input as a half-open byte range.
	Pos, End int
	// Depth is the parenthesis nesting level at the token's start. A
	// top-level clause has Depth 0; anything inside a subquery is deeper.
	Depth int
}

// IsKeyword reports whether the token is a bare word equal to name,
// case-insensitively. Backtick-quoted identifiers never match, which is what
// makes `SELECT ` + "`where`" + ` FROM t` safe to analyse.
func (t Token) IsKeyword(name string) bool {
	return t.Kind == Word && strings.EqualFold(t.Text, name)
}

// Tokenize splits sql into tokens.
//
// Version-hint comments (/*! ... */) are unwrapped rather than skipped:
// MySQL executes their contents, so guard has to see them.
func Tokenize(sql string) []Token {
	l := &lexer{src: sql}
	l.run(0)
	return l.toks
}

type lexer struct {
	src   string
	toks  []Token
	depth int
}

// run scans from offset i to the end of src, appending tokens.
func (l *lexer) run(i int) {
	for i < len(l.src) {
		c := l.src[i]

		switch {
		case isSpace(c):
			i++

		case c == '\'' || c == '"':
			i = l.scanQuoted(i, c, String)

		case c == '`':
			i = l.scanQuoted(i, '`', Ident)

		case c == '#':
			i = l.scanLineComment(i)

		case c == '-' && strings.HasPrefix(l.src[i:], "--"):
			i = l.scanLineComment(i)

		case c == '/' && strings.HasPrefix(l.src[i:], "/*"):
			i = l.scanBlockComment(i)

		case isDigit(c):
			i = l.scanNumber(i)

		case isWordByte(c):
			i = l.scanWord(i)

		default:
			// Track nesting before emitting so an opening parenthesis sits
			// at the outer depth and its contents one level deeper.
			if c == '(' {
				l.emit(Punct, i, i+1)
				l.depth++
			} else if c == ')' {
				if l.depth > 0 {
					l.depth--
				}
				l.emit(Punct, i, i+1)
			} else {
				l.emit(Punct, i, i+1)
			}
			i++
		}
	}
}

func (l *lexer) emit(k Kind, pos, end int) {
	l.toks = append(l.toks, Token{
		Kind:  k,
		Text:  l.src[pos:end],
		Pos:   pos,
		End:   end,
		Depth: l.depth,
	})
}

// scanQuoted consumes a literal delimited by quote, honouring both the
// doubled-delimiter and backslash escape forms. An unterminated literal runs
// to end of input rather than resynchronising, so the caller sees one token
// instead of misreading the rest of the statement as code.
func (l *lexer) scanQuoted(start int, quote byte, k Kind) int {
	i := start + 1
	for i < len(l.src) {
		switch l.src[i] {
		case '\\':
			// Backslash escapes do not apply inside backticks.
			if quote == '`' {
				i++
				continue
			}
			i += 2
		case quote:
			if i+1 < len(l.src) && l.src[i+1] == quote {
				i += 2
				continue
			}
			l.emit(k, start, i+1)
			return i + 1
		default:
			i++
		}
	}
	l.emit(k, start, len(l.src))
	return len(l.src)
}

func (l *lexer) scanLineComment(start int) int {
	i := strings.IndexByte(l.src[start:], '\n')
	if i < 0 {
		l.emit(Comment, start, len(l.src))
		return len(l.src)
	}
	l.emit(Comment, start, start+i)
	return start + i
}

// scanBlockComment handles three shapes that look alike but differ in
// meaning: /* ordinary */, /*+ optimizer hint */ and /*! executed by MySQL */.
func (l *lexer) scanBlockComment(start int) int {
	end := strings.Index(l.src[start+2:], "*/")
	closed := end >= 0

	var body, next int
	if closed {
		body = start + 2 + end
		next = body + 2
	} else {
		body = len(l.src)
		next = len(l.src)
	}

	if l.isExecutableComment(start) {
		// Skip "/*!" and any version digits, then lex the contents in place
		// so the statement reads exactly as the server will execute it.
		inner := start + 3
		for inner < body && isDigit(l.src[inner]) {
			inner++
		}
		saved := l.src
		l.src = l.src[:body]
		l.run(inner)
		l.src = saved
		return next
	}

	l.emit(Comment, start, next)
	return next
}

// isExecutableComment distinguishes /*! from the optimizer hint /*+.
func (l *lexer) isExecutableComment(start int) bool {
	return start+2 < len(l.src) && l.src[start+2] == '!'
}

func (l *lexer) scanNumber(start int) int {
	i := start
	for i < len(l.src) && (isDigit(l.src[i]) || l.src[i] == '.') {
		i++
	}
	l.emit(Number, start, i)
	return i
}

func (l *lexer) scanWord(start int) int {
	i := start
	for i < len(l.src) && isWordByte(l.src[i]) {
		i++
	}
	l.emit(Word, start, i)
	return i
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// IsIdentifierByte reports whether a byte can appear in an unquoted MySQL
// identifier.
//
// It is exported so that cursor movement in the editor uses the same notion
// of a word as the tokenizer does. Without that, "user_id" would be one word
// to completion and three to the arrow keys, and the difference would be
// impossible for a user to account for.
func IsIdentifierByte(c byte) bool { return isWordByte(c) }

// isWordByte accepts the bytes MySQL allows in an unquoted identifier, plus
// everything above ASCII so that multi-byte names stay in one token.
func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		isDigit(c) ||
		c == '_' || c == '$' ||
		c >= 0x80
}
