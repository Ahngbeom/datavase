package sqlparse

import "strings"

// CompletionKind says what sort of name belongs at the caret.
type CompletionKind int

const (
	// CompleteNone means nothing should be offered — inside a literal, a
	// comment, or before any clause has established a context.
	CompleteNone CompletionKind = iota
	// CompleteTable means a table name is expected.
	CompleteTable
	// CompleteColumn means a column of the statement's tables is expected.
	CompleteColumn
	// CompleteQualified means the caret follows "something.", where the
	// qualifier is either a table alias or a schema name.
	CompleteQualified
)

// TableRef is one entry of a FROM, JOIN, UPDATE or INSERT clause.
type TableRef struct {
	Schema string
	Name   string
	Alias  string
	// Derived marks a subquery: it has an alias but no name to look up.
	Derived bool
}

// CompletionContext describes what to offer at a caret position.
type CompletionContext struct {
	Kind CompletionKind
	// Prefix is the partial identifier already typed.
	Prefix string
	// Qualifier is the name before the dot, when Kind is CompleteQualified.
	Qualifier string
	// Tables are the statement's tables, so a qualifier can be resolved
	// without parsing again.
	Tables []TableRef
	// ReplaceFrom and ReplaceTo delimit the text a chosen candidate replaces.
	ReplaceFrom, ReplaceTo int
}

// CompletionAt analyses the caret position within sql.
//
// The whole statement is parsed, not just the text before the caret: in
// "SELECT u.| FROM users u" the meaning of "u" is established after the
// caret, and looking only backwards would leave it unresolvable.
func CompletionAt(sql string, offset int) CompletionContext {
	offset = clampOffset(offset, len(sql))

	stmt, ok := StatementAt(sql, offset)
	if !ok {
		return CompletionContext{ReplaceFrom: offset, ReplaceTo: offset}
	}

	ctx := CompletionContext{
		Tables:      TableRefs(stmt),
		ReplaceFrom: offset,
		ReplaceTo:   offset,
	}

	// A caret inside a literal or a comment is not naming anything.
	if enclosingToken(stmt, offset) != nil {
		return ctx
	}

	// Positions are absolute offsets into sql throughout. Token positions are
	// already recorded that way, while stmt.SQL is only a slice of it — mixing
	// the two goes out of range the moment the caret sits in the whitespace
	// after a statement, which is exactly where "FROM |" puts it.
	word, dot := wordBefore(sql, offset)
	ctx.Prefix = word.text
	ctx.ReplaceFrom = word.start
	ctx.ReplaceTo = offset

	if dot != "" {
		ctx.Kind = CompleteQualified
		ctx.Qualifier = dot
		return ctx
	}

	ctx.Kind = clauseKind(stmt, word.start)
	return ctx
}

func clampOffset(offset, length int) int {
	if offset < 0 {
		return 0
	}
	if offset > length {
		return length
	}
	return offset
}

// enclosingToken returns the literal or comment the offset sits inside, if
// any. Completion inside one would be nonsense.
func enclosingToken(stmt Statement, offset int) *Token {
	for i := range stmt.Tokens {
		tk := &stmt.Tokens[i]
		if tk.Kind != String && tk.Kind != Comment {
			continue
		}
		if offset > tk.Pos && offset <= tk.End {
			return tk
		}
	}
	return nil
}

// typedWord is the partial identifier under the caret.
type typedWord struct {
	text  string
	start int // absolute offset in the source
}

// wordBefore returns the identifier being typed and, when the caret follows
// "name.", that qualifier.
func wordBefore(sql string, offset int) (typedWord, string) {
	// Walk back over identifier characters to find what has been typed.
	start := offset
	for start > 0 && isWordByte(sql[start-1]) {
		start--
	}
	word := typedWord{text: sql[start:offset], start: start}

	// A dot immediately before makes this a qualified reference.
	if start == 0 || sql[start-1] != '.' {
		return word, ""
	}

	return word, identifierEndingAt(sql, start-1)
}

// identifierEndingAt reads the identifier that ends just before at, handling
// the backtick-quoted form.
func identifierEndingAt(sql string, at int) string {
	if at > 0 && sql[at-1] == '`' {
		if open := strings.LastIndexByte(sql[:at-1], '`'); open >= 0 {
			return sql[open+1 : at-1]
		}
		return ""
	}

	start := at
	for start > 0 && isWordByte(sql[start-1]) {
		start--
	}
	return sql[start:at]
}

// clauseKeywords map a clause introducer to what may follow it.
var clauseKeywords = map[string]CompletionKind{
	"SELECT": CompleteColumn,
	"WHERE":  CompleteColumn,
	"HAVING": CompleteColumn,
	"ON":     CompleteColumn,
	"SET":    CompleteColumn,
	"BY":     CompleteColumn, // ORDER BY, GROUP BY
	"AND":    CompleteColumn,
	"OR":     CompleteColumn,
	"FROM":   CompleteTable,
	"JOIN":   CompleteTable,
	"INTO":   CompleteTable,
	"UPDATE": CompleteTable,
	"TABLE":  CompleteTable,
}

// clauseKind decides what belongs at a position by finding the most recent
// clause keyword before it.
func clauseKind(stmt Statement, offset int) CompletionKind {
	for i := len(stmt.Tokens) - 1; i >= 0; i-- {
		tk := stmt.Tokens[i]
		if tk.Kind != Word || tk.End > offset {
			continue
		}
		if kind, ok := clauseKeywords[strings.ToUpper(tk.Text)]; ok {
			return kind
		}
	}
	return CompleteNone
}

// TableRefs lists the tables a statement reads or writes.
//
// Only the outermost level is walked. A derived table contributes its alias
// so that "s." resolves to something, but its inner tables are not in scope
// for the outer query and must not be offered.
func TableRefs(stmt Statement) []TableRef {
	var (
		refs      []TableRef
		expecting bool
	)

	for i := 0; i < len(stmt.Tokens); i++ {
		tk := stmt.Tokens[i]
		if tk.Kind == Comment {
			continue
		}

		if tk.Depth == 0 && tk.Kind == Word && introducesTable(tk.Text) {
			expecting = true
			continue
		}
		if !expecting {
			continue
		}

		// A parenthesis here opens a derived table; skip to its alias.
		if tk.Kind == Punct && tk.Text == "(" {
			ref, next := readDerived(stmt.Tokens, i)
			refs = append(refs, ref)
			i = next
			expecting = false
			continue
		}

		if tk.Depth != 0 || (tk.Kind != Word && tk.Kind != Ident) {
			expecting = false
			continue
		}

		ref, next := readTableRef(stmt.Tokens, i)
		refs = append(refs, ref)
		i = next

		// A comma continues the list; anything else ends it.
		expecting = i+1 < len(stmt.Tokens) &&
			stmt.Tokens[i+1].Kind == Punct && stmt.Tokens[i+1].Text == ","
		if expecting {
			i++
		}
	}
	return refs
}

// introducesTable reports whether the keyword is followed by a table name.
func introducesTable(word string) bool {
	switch strings.ToUpper(word) {
	case "FROM", "JOIN", "INTO", "UPDATE":
		return true
	default:
		return false
	}
}

// readTableRef reads "schema.name alias" starting at index i and returns the
// index of its last token.
func readTableRef(tokens []Token, i int) (TableRef, int) {
	var ref TableRef
	ref.Name = identifierText(tokens[i])

	// An optional schema qualifier.
	if i+2 < len(tokens) && tokens[i+1].Kind == Punct && tokens[i+1].Text == "." {
		ref.Schema = ref.Name
		ref.Name = identifierText(tokens[i+2])
		i += 2
	}

	// An optional alias, with or without AS.
	next := i + 1
	if next < len(tokens) && tokens[next].IsKeyword("AS") {
		next++
	}
	if next < len(tokens) && isAliasToken(tokens[next]) {
		ref.Alias = identifierText(tokens[next])
		i = next
	}
	return ref, i
}

// readDerived skips a parenthesised subquery and takes its alias.
func readDerived(tokens []Token, open int) (TableRef, int) {
	depth := tokens[open].Depth

	i := open + 1
	for i < len(tokens) {
		tk := tokens[i]
		if tk.Kind == Punct && tk.Text == ")" && tk.Depth == depth {
			break
		}
		i++
	}

	ref := TableRef{Derived: true}
	next := i + 1
	if next < len(tokens) && tokens[next].IsKeyword("AS") {
		next++
	}
	if next < len(tokens) && isAliasToken(tokens[next]) {
		ref.Alias = identifierText(tokens[next])
		i = next
	}
	return ref, i
}

// isAliasToken reports whether a token can be a table alias. A keyword that
// starts the next clause is not one — treating WHERE as an alias would make
// every unaliased table look aliased.
func isAliasToken(tk Token) bool {
	if tk.Kind == Ident {
		return true
	}
	if tk.Kind != Word {
		return false
	}
	return !reservedAfterTable[strings.ToUpper(tk.Text)]
}

// reservedAfterTable are the words that may follow a table name without
// being its alias.
var reservedAfterTable = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "JOIN": true, "INNER": true,
	"LEFT": true, "RIGHT": true, "FULL": true, "CROSS": true,
	"OUTER": true, "STRAIGHT_JOIN": true, "ON": true, "USING": true,
	"SET": true, "VALUES": true, "SELECT": true, "UNION": true,
	"FOR": true, "LOCK": true, "INTO": true, "PARTITION": true,
	"AS": true, "USE": true, "IGNORE": true, "FORCE": true,
	"WITH": true, "NATURAL": true,
}

// identifierText strips the quoting from a backtick-quoted identifier.
func identifierText(tk Token) string {
	if tk.Kind == Ident && len(tk.Text) >= 2 {
		return strings.ReplaceAll(tk.Text[1:len(tk.Text)-1], "``", "`")
	}
	return tk.Text
}

// ResolveQualifier finds the table a qualifier stands for.
//
// An alias wins; failing that, a table answers to its own name, which is what
// makes "users.id" work in a statement that never aliased users.
func ResolveQualifier(refs []TableRef, qualifier string) (TableRef, bool) {
	for _, ref := range refs {
		if ref.Alias != "" && strings.EqualFold(ref.Alias, qualifier) {
			return ref, true
		}
	}
	for _, ref := range refs {
		if ref.Alias == "" && strings.EqualFold(ref.Name, qualifier) {
			return ref, true
		}
	}
	return TableRef{}, false
}
