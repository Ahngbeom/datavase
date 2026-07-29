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
	// StmtSession changes connection state: USE, SET.
	StmtSession
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

// Kind reports what the statement does.
func (s Statement) Kind() StmtKind {
	first, ok := s.firstWord()
	if !ok || first.Kind != Word {
		return StmtOther
	}

	switch strings.ToUpper(first.Text) {
	case "SELECT", "WITH", "TABLE", "VALUES":
		return StmtSelect
	case "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "ANALYZE", "CHECK":
		return StmtRead
	case "USE", "SET":
		return StmtSession
	case "INSERT", "REPLACE":
		return StmtInsert
	case "UPDATE":
		return StmtUpdate
	case "DELETE":
		return StmtDelete
	case "TRUNCATE":
		return StmtTruncate
	case "DROP":
		return StmtDrop
	case "CREATE", "ALTER", "RENAME":
		return StmtDDL
	default:
		return StmtOther
	}
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
