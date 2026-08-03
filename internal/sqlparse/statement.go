package sqlparse

import "strings"

// StmtKind classifies a statement by what it does to the server.
type StmtKind int

const (
	// StmtOther is anything not recognised. Guard treats it as unsafe.
	StmtOther StmtKind = iota
	// StmtSelect reads rows.
	StmtSelect
	// StmtRead is a non-SELECT read: SHOW, DESCRIBE, EXPLAIN.
	StmtRead
	// StmtSession changes connection state: USE, SET, LOCK TABLES.
	StmtSession
	// StmtTransaction opens, ends or marks a transaction. It is separate from
	// StmtSession because the caller's answer differs: session state is lost,
	// whereas an abandoned transaction can hold locks after it is lost.
	StmtTransaction
	// StmtInsert adds rows (INSERT, REPLACE).
	StmtInsert
	// StmtUpdate modifies rows.
	StmtUpdate
	// StmtDelete removes rows and can be bounded by WHERE.
	StmtDelete
	// StmtTruncate empties a table and cannot be bounded or rolled back.
	StmtTruncate
	// StmtDrop removes a schema object.
	StmtDrop
	// StmtDDL is any other schema change: CREATE, ALTER, RENAME.
	StmtDDL
)

func (k StmtKind) String() string {
	switch k {
	case StmtSelect:
		return "SELECT"
	case StmtRead:
		return "read"
	case StmtSession:
		return "session"
	case StmtTransaction:
		return "transaction control"
	case StmtInsert:
		return "INSERT"
	case StmtUpdate:
		return "UPDATE"
	case StmtDelete:
		return "DELETE"
	case StmtTruncate:
		return "TRUNCATE"
	case StmtDrop:
		return "DROP"
	case StmtDDL:
		return "DDL"
	default:
		return "unrecognised"
	}
}

// ReturnsRows reports whether a statement of this kind answers with a result
// set rather than with a count of what it changed.
//
// The decision has to be made before the statement is sent: a write sent as a
// query yields a result set with no columns, and the count the server reported
// is gone by the time anyone could ask for it.
//
// Anything not definitely a write is treated as returning rows. Sending a
// query as a query costs nothing when it turns out to have no rows, whereas
// sending something like CALL as a write discards the rows it did produce —
// so the uncertain case goes the way that cannot lose anything.
func (k StmtKind) ReturnsRows() bool {
	switch k {
	case StmtInsert, StmtUpdate, StmtDelete, StmtTruncate, StmtDrop, StmtDDL:
		return false
	default:
		return true
	}
}

// Statement is one SQL statement with its tokens and its span in the source
// buffer. The span lets the editor highlight exactly what will run.
type Statement struct {
	SQL      string
	Pos, End int
	Tokens   []Token
}

// Parse tokenizes a single statement.
func Parse(sql string) Statement {
	return Statement{SQL: sql, End: len(sql), Tokens: Tokenize(sql)}
}

// IsEmpty reports whether the statement carries no executable text.
func (s Statement) IsEmpty() bool {
	for _, tk := range s.Tokens {
		if tk.Kind != Comment {
			return false
		}
	}
	return true
}

// firstWord returns the first token that is not a comment.
func (s Statement) firstWord() (Token, bool) {
	for _, tk := range s.Tokens {
		if tk.Kind == Comment {
			continue
		}
		// A leading parenthesis, as in "(SELECT 1) UNION (SELECT 2)", is
		// not itself the verb.
		if tk.Kind == Punct && tk.Text == "(" {
			continue
		}
		return tk, true
	}
	return Token{}, false
}

// kindIfSecondWordIs resolves a verb that starts more than one kind of
// statement, falling back to StmtOther so the guard's fail-closed default
// covers whatever else the verb might have begun.
func kindIfSecondWordIs(s Statement, want string, kind StmtKind) StmtKind {
	var seen bool
	for _, tk := range s.Tokens {
		if tk.Kind != Word {
			continue
		}
		if !seen {
			seen = true
			continue
		}
		if strings.EqualFold(tk.Text, want) {
			return kind
		}
		return StmtOther
	}
	return StmtOther
}

// Kind reports what the statement does.
func (s Statement) Kind() StmtKind {
	first, ok := s.firstWord()
	if !ok || first.Kind != Word {
		return StmtOther
	}

	switch word := strings.ToUpper(first.Text); word {
	case "SHOW", "DESCRIBE", "DESC", "CHECK":
		return StmtRead

	// EXPLAIN and ANALYZE both wrap another statement, and one of them runs
	// it. See wrappedKind.
	case "EXPLAIN", "ANALYZE":
		return s.wrappedKind(word)
	case "USE", "SET", "LOCK", "UNLOCK":
		return StmtSession
	case "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT":
		return StmtTransaction

	// START and RELEASE each begin two unrelated statements, and the wrong
	// one would be handed an explanation about transactions while escaping
	// the fail-closed default meant for anything unrecognised.
	case "START":
		return kindIfSecondWordIs(s, "TRANSACTION", StmtTransaction)
	case "RELEASE":
		return kindIfSecondWordIs(s, "SAVEPOINT", StmtTransaction)

	default:
		// Every verb whose kind its first word settles. The same table
		// answers for a wrapped statement, so a verb added here cannot be
		// classified one way on its own and another behind an ANALYZE.
		if kind, known := verbKinds[word]; known {
			return kind
		}
		return StmtOther
	}
}

// wrappedKind classifies a statement that begins EXPLAIN or ANALYZE.
//
// The two are not the same question. EXPLAIN reports how a statement *would*
// run and executes nothing, so it is a read whatever it wraps. ANALYZE — and
// MySQL's spelling of it, EXPLAIN ANALYZE — **runs the statement** and reports
// what actually happened, so it is exactly as dangerous as what follows it.
// "ANALYZE FORMAT=JSON DELETE FROM orders" empties the table.
//
// A running wrapper therefore takes the kind of the statement it wraps, and
// the guard reasons about that: HasTopLevelWhere scans at parenthesis depth
// zero, so a bounded delete stays bounded through the prefix.
//
// Anything wrapping a verb this package does not recognise is StmtOther, which
// is the fail-closed default and the reason kindIfSecondWordIs exists for
// START and RELEASE.
func (s Statement) wrappedKind(verb string) StmtKind {
	inner, ok := s.wrappedStatement(verb)
	if !ok {
		// EXPLAIN with nothing recognisable after it is the plain form:
		// "EXPLAIN orders", "EXPLAIN FOR CONNECTION 12", or a plan of a
		// statement whose verb is not one we classify.
		if verb == "EXPLAIN" {
			return StmtRead
		}
		return StmtOther
	}
	return inner
}

// analyzeTargets are the words that make ANALYZE a maintenance statement
// rather than a wrapper. It rewrites a table's statistics, not its rows.
var analyzeTargets = map[string]bool{
	"TABLE": true, "LOCAL": true, "NO_WRITE_TO_BINLOG": true,
}

// wrappedStatement finds the kind of the statement a running wrapper carries,
// reporting whether there was one.
func (s Statement) wrappedStatement(verb string) (StmtKind, bool) {
	runs := verb == "ANALYZE"

	for _, tk := range s.words()[1:] {
		word := strings.ToUpper(tk)

		// EXPLAIN only runs its statement when ANALYZE says so.
		if word == "ANALYZE" {
			runs = true
			continue
		}
		if runs && analyzeTargets[word] {
			return StmtRead, true
		}
		if kind, known := verbKinds[word]; known {
			if !runs {
				return 0, false
			}
			return kind, true
		}
	}
	return 0, false
}

// words returns the statement's word tokens as text, which is all the wrapper
// scan needs: the prefixes it steps over — FORMAT, JSON, EXTENDED — carry no
// parentheses, so nothing here has to track depth.
func (s Statement) words() []string {
	var out []string
	for _, tk := range s.Tokens {
		if tk.Kind == Word {
			out = append(out, tk.Text)
		}
	}
	return out
}

// verbKinds are the statement verbs a wrapper can be carrying. Only the ones
// whose kind is decided by the verb alone: START and RELEASE need a second
// word and are never wrapped.
var verbKinds = map[string]StmtKind{
	"SELECT": StmtSelect, "WITH": StmtSelect, "TABLE": StmtSelect, "VALUES": StmtSelect,
	"INSERT": StmtInsert, "REPLACE": StmtInsert,
	"UPDATE":   StmtUpdate,
	"DELETE":   StmtDelete,
	"TRUNCATE": StmtTruncate,
	"DROP":     StmtDrop,
	"CREATE":   StmtDDL, "ALTER": StmtDDL, "RENAME": StmtDDL,
}

// Verb is the statement's leading keyword, upper-cased, or "" if it has none.
// Kind is the right question almost everywhere; this exists for the few
// places that have to tell two statements of one kind apart.
func (s Statement) Verb() string {
	first, ok := s.firstWord()
	if !ok || first.Kind != Word {
		return ""
	}
	return strings.ToUpper(first.Text)
}

// HasTopLevelWhere reports whether the statement is bounded by a WHERE
// clause of its own, ignoring any that belong to subqueries.
func (s Statement) HasTopLevelWhere() bool {
	return s.hasTopLevelKeyword("WHERE")
}

// HasTopLevelLimit reports whether the statement already limits its result,
// which is what stops the auto-limit from overriding an explicit choice.
func (s Statement) HasTopLevelLimit() bool {
	return s.hasTopLevelKeyword("LIMIT")
}

func (s Statement) hasTopLevelKeyword(name string) bool {
	for _, tk := range s.Tokens {
		if tk.Depth == 0 && tk.IsKeyword(name) {
			return true
		}
	}
	return false
}

// Split breaks sql into statements on unquoted semicolons. Blank statements
// and comment-only fragments are dropped.
func Split(sql string) []Statement {
	toks := Tokenize(sql)

	var (
		stmts []Statement
		start int
		group []Token
	)

	flush := func(end int) {
		s := trimSpan(sql, start, end, group)
		if !s.IsEmpty() {
			stmts = append(stmts, s)
		}
		group = nil
	}

	for _, tk := range toks {
		if tk.Kind == Punct && tk.Text == ";" && tk.Depth == 0 {
			flush(tk.Pos)
			start = tk.End
			continue
		}
		group = append(group, tk)
	}
	flush(len(sql))

	return stmts
}

// trimSpan narrows [start,end) to the statement's own text, dropping the
// surrounding whitespace so the recorded span matches SQL exactly.
func trimSpan(sql string, start, end int, toks []Token) Statement {
	if len(toks) > 0 {
		start = toks[0].Pos
		end = toks[len(toks)-1].End
	}
	if start > end {
		start = end
	}
	return Statement{SQL: sql[start:end], Pos: start, End: end, Tokens: toks}
}

// StatementAt returns the statement containing the byte offset, which is how
// Ctrl+Enter decides what to run.
//
// Whitespace between statements attaches to the preceding one: a cursor
// resting just after "SELECT 1;" belongs to that statement, not to whatever
// comes next. Running the following statement there would be a surprise.
func StatementAt(sql string, offset int) (Statement, bool) {
	stmts := Split(sql)
	if len(stmts) == 0 {
		return Statement{}, false
	}

	for i, s := range stmts {
		if offset <= s.End {
			return s, true
		}
		if i+1 < len(stmts) && offset < stmts[i+1].Pos {
			return s, true
		}
	}
	return stmts[len(stmts)-1], true
}
