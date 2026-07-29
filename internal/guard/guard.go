// Package guard decides whether a statement may run against a datasource.
//
// Evaluate is a pure function of the statement and the policy. It never
// touches a database, which is what allows the whole policy surface to be
// covered by fast table-driven tests — and what makes it trustworthy.
//
// The governing rule is fail-closed: sqlparse is a tokenizer, not a full
// MySQL parser, so anything it cannot classify is refused in production
// rather than assumed harmless.
package guard

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// Verdict is what the runner should do with a statement.
type Verdict int

const (
	// Allow runs the statement immediately.
	Allow Verdict = iota
	// Confirm runs it only after the user agrees.
	Confirm
	// Deny refuses to run it at all.
	Deny
)

func (v Verdict) String() string {
	switch v {
	case Allow:
		return "allow"
	case Confirm:
		return "confirm"
	case Deny:
		return "deny"
	default:
		return "unknown"
	}
}

// Policy is the rule set in force for one datasource.
type Policy struct {
	// Env decides how strict the rules are.
	Env config.Env
	// AutoLimit is appended to unbounded SELECTs. Zero disables it.
	AutoLimit int
	// WritesEnabled records the session-level ":write" opt-in. It only ever
	// relaxes a Deny into a Confirm, and never for unbounded or destructive
	// statements.
	WritesEnabled bool
}

// Decision is the result of evaluating one statement.
type Decision struct {
	Verdict Verdict
	// Reason explains the verdict; the status bar and dialog show it.
	Reason string
	// TypeToConfirm, when set, is the phrase the user must type out before
	// the statement runs. Reserved for irreversible operations, where a
	// reflexive "yes" is the failure mode being defended against.
	TypeToConfirm string
	// InjectLimit is the LIMIT to append, or zero to leave the SQL alone.
	InjectLimit int
}

// Evaluate applies p to stmt.
func Evaluate(stmt sqlparse.Statement, p Policy) Decision {
	if stmt.IsEmpty() {
		return Decision{Verdict: Deny, Reason: "nothing to run"}
	}

	kind := stmt.Kind()
	prod := p.Env == config.EnvProd

	switch kind {
	case sqlparse.StmtSelect:
		return Decision{Verdict: Allow, InjectLimit: p.autoLimitFor(stmt)}

	case sqlparse.StmtRead, sqlparse.StmtSession:
		return Decision{Verdict: Allow}

	case sqlparse.StmtUpdate, sqlparse.StmtDelete:
		return evaluateRowWrite(stmt, kind, p, prod)

	case sqlparse.StmtInsert:
		return gateWrite(p, prod, fmt.Sprintf("%s modifies data", kind))

	case sqlparse.StmtTruncate, sqlparse.StmtDrop:
		return evaluateDestructive(kind, prod)

	case sqlparse.StmtDDL:
		if prod {
			return Decision{
				Verdict: Deny,
				Reason:  fmt.Sprintf("%s changes the schema of a production database", kind),
			}
		}
		return Decision{
			Verdict: Confirm,
			Reason:  fmt.Sprintf("%s changes the schema", kind),
		}

	default:
		// Unrecognised. It might be harmless; it might be GRANT.
		if prod {
			return Decision{
				Verdict: Deny,
				Reason:  "this statement could not be classified, so it is refused against production",
			}
		}
		return Decision{
			Verdict: Confirm,
			Reason:  "this statement could not be classified",
		}
	}
}

// evaluateRowWrite handles UPDATE and DELETE, where the presence of a
// top-level WHERE is the difference between editing rows and rewriting a
// table.
func evaluateRowWrite(stmt sqlparse.Statement, kind sqlparse.StmtKind, p Policy, prod bool) Decision {
	if !stmt.HasTopLevelWhere() {
		if prod {
			return Decision{
				Verdict: Deny,
				Reason: fmt.Sprintf(
					"%s without a top-level WHERE would affect every row of a production table",
					kind),
			}
		}
		return Decision{
			Verdict:       Confirm,
			Reason:        fmt.Sprintf("%s without a WHERE clause affects every row", kind),
			TypeToConfirm: confirmPhrase(kind),
		}
	}
	return gateWrite(p, prod, fmt.Sprintf("%s modifies data", kind))
}

// evaluateDestructive handles TRUNCATE and DROP, which cannot be narrowed by
// a WHERE clause and cannot be rolled back.
func evaluateDestructive(kind sqlparse.StmtKind, prod bool) Decision {
	if prod {
		return Decision{
			Verdict: Deny,
			Reason:  fmt.Sprintf("%s destroys production data and cannot be rolled back", kind),
		}
	}
	return Decision{
		Verdict:       Confirm,
		Reason:        fmt.Sprintf("%s cannot be rolled back", kind),
		TypeToConfirm: confirmPhrase(kind),
	}
}

// gateWrite applies the production write lock to an otherwise ordinary write.
func gateWrite(p Policy, prod bool, reason string) Decision {
	if prod && !p.WritesEnabled {
		return Decision{
			Verdict: Deny,
			Reason:  reason + "; writes to production are locked (unlock with :write)",
		}
	}
	return Decision{Verdict: Confirm, Reason: reason}
}

func (p Policy) autoLimitFor(stmt sqlparse.Statement) int {
	if p.AutoLimit <= 0 || stmt.HasTopLevelLimit() {
		return 0
	}
	return p.AutoLimit
}

func confirmPhrase(kind sqlparse.StmtKind) string {
	return kind.String()
}
