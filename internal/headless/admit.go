// Package headless runs one read statement without a terminal and writes the
// result as JSON lines, for a program to read rather than a person.
//
// It refuses anything it cannot show to be a plain read, before it is sent,
// and it is only ever handed a read_only datasource, so the server's own
// refusal stands behind every check here. The checks are a guard against a
// slip, as read_only is: the database account is the boundary.
//
// It knows nothing of who is asking or which columns they may see. That
// policy belongs to whatever runs dv, which is also where it can change
// without a release of this program.
package headless

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// Refusal is a statement turned away before it reached the server.
type Refusal struct {
	Reason string
}

func (r *Refusal) Error() string { return r.Reason }

func refuse(format string, args ...any) *Refusal {
	return &Refusal{Reason: fmt.Sprintf(format, args...)}
}

// Admit returns the one statement in sql if it is a plain read, and a
// *Refusal saying why not otherwise.
func Admit(sql string) (sqlparse.Statement, error) {
	stmts := sqlparse.Split(sql)
	switch len(stmts) {
	case 0:
		return sqlparse.Statement{}, refuse("there is no statement to run")
	case 1:
	default:
		// A second statement is the one no caller looked at.
		return sqlparse.Statement{}, refuse("dv query runs one statement at a time; got %d", len(stmts))
	}
	stmt := stmts[0]

	// The tokenizer unwraps these so that Kind sees what the server will run,
	// which makes everything below depend on it having unwrapped them exactly
	// as the server does. Refusing them outright removes that dependency.
	if hasExecutableComment(sql) {
		return sqlparse.Statement{}, refuse("the statement holds an executable comment (/*! … */), which the server runs as SQL")
	}

	if err := admitKind(stmt); err != nil {
		return sqlparse.Statement{}, err
	}
	if err := admitWords(stmt); err != nil {
		return sqlparse.Statement{}, err
	}
	return stmt, nil
}

// readVerbs are the non-SELECT reads admitted. CHECK is a read by kind, but
// it walks a whole table and can hold it while doing so.
var readVerbs = map[string]bool{"SHOW": true, "DESCRIBE": true, "DESC": true, "EXPLAIN": true}

func admitKind(stmt sqlparse.Statement) error {
	verb := stmt.Verb()

	// EXPLAIN ANALYZE takes the kind of the SELECT it wraps, and ANALYZE
	// TABLE is a read by kind; both do work the caller did not ask to have
	// bounded, so the word settles it before the kind is consulted.
	if (verb == "EXPLAIN" || verb == "ANALYZE") && containsWord(stmt, "ANALYZE") {
		if kind := stmt.Kind(); kind != sqlparse.StmtSelect && kind != sqlparse.StmtRead {
			return refuse("%s statements are not run here; only SELECT, SHOW, DESCRIBE and EXPLAIN are", kind)
		}
		return refuse("ANALYZE runs the statement it names in full; use EXPLAIN without it for the plan")
	}

	switch kind := stmt.Kind(); kind {
	case sqlparse.StmtSelect:
		return nil
	case sqlparse.StmtRead:
		if readVerbs[verb] {
			return nil
		}
		return refuse("%s is not run here; only SELECT, SHOW, DESCRIBE and EXPLAIN are", verb)
	default:
		return refuse("%s statements are not run here; only SELECT, SHOW, DESCRIBE and EXPLAIN are", kind)
	}
}

// refusedCalls are functions that hold the connection, take a lock outside
// any transaction, or read a file on the database host.
var refusedCalls = map[string]string{
	"SLEEP":     "SLEEP() holds the connection without reading anything",
	"BENCHMARK": "BENCHMARK() keeps the server busy without reading anything",
	"GET_LOCK":  "GET_LOCK() takes a named lock that outlives the statement",
	"LOAD_FILE": "LOAD_FILE() reads a file on the database host",
}

// admitWords looks for the clauses that turn a SELECT into something else.
//
// Only bare words are inspected, so a literal, a quoted name or a comment
// that happens to spell one of them is never mistaken for it.
func admitWords(stmt sqlparse.Statement) error {
	toks := stmt.Tokens
	for i, tk := range toks {
		if tk.Kind != sqlparse.Word {
			continue
		}
		word := strings.ToUpper(tk.Text)
		calls := i+1 < len(toks) && toks[i+1].Kind == sqlparse.Punct && toks[i+1].Text == "("
		afterFor := i > 0 && toks[i-1].IsKeyword("FOR")

		switch {
		case word == "INTO":
			return refuse("SELECT … INTO sends the result somewhere other than here")
		case (word == "UPDATE" || word == "SHARE") && afterFor:
			return refuse("FOR %s locks the rows it reads", word)
		case word == "LOCK" && i+1 < len(toks) && toks[i+1].IsKeyword("IN"):
			return refuse("LOCK IN SHARE MODE locks the rows it reads")
		// WITH can lead an UPDATE or a DELETE, which the verb table reads as
		// a SELECT. INSERT and REPLACE are also string functions.
		case word == "UPDATE" || word == "DELETE" || ((word == "INSERT" || word == "REPLACE") && !calls):
			return refuse("the statement carries a write (%s) behind its first word", word)
		case calls && refusedCalls[word] != "":
			return refuse("%s", refusedCalls[word])
		}
	}
	return nil
}

func containsWord(stmt sqlparse.Statement, word string) bool {
	for _, tk := range stmt.Tokens {
		if tk.IsKeyword(word) {
			return true
		}
	}
	return false
}

// hasExecutableComment reports whether "/*!" opens anywhere outside a token.
//
// The tokenizer leaves no token for the comment markers themselves, so they
// can only be found in the gaps; inside a literal or an ordinary comment the
// same three characters are part of a token and mean nothing.
func hasExecutableComment(sql string) bool {
	at := 0
	for _, tk := range sqlparse.Tokenize(sql) {
		if tk.Pos > at && strings.Contains(sql[at:tk.Pos], "/*!") {
			return true
		}
		if tk.End > at {
			at = tk.End
		}
	}
	return strings.Contains(sql[at:], "/*!")
}
