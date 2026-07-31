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
	// Unlockable reports that the production write lock is the only thing in
	// the way, so the caller may offer the way past it.
	//
	// It is a flag rather than a sentence because guard must not name a route
	// through an interface it cannot see: the reason once read "unlock with
	// :write", which was a command no preset had. Whoever draws the dialog
	// knows what the keys are; this package does not.
	Unlockable bool
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

	case sqlparse.StmtRead:
		return Decision{Verdict: Allow}

	case sqlparse.StmtTransaction, sqlparse.StmtSession:
		return refuseUnheldState(stmt, kind)

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

// refuseUnheldState handles the statements whose whole effect lives on the
// connection that ran them.
//
// Statements are run on a connection borrowed from the pool and handed back
// when they finish, so none of this reaches the next statement: BEGIN opens a
// transaction nothing will commit, SET SESSION is accepted and discarded, and
// ROLLBACK reports success having undone nothing. Every one of them looks
// like it worked, which is worse than being refused — an abandoned
// transaction can also sit on the pooled connection holding locks.
//
// This is the fail-closed rule reaching a case that used to escape it: these
// were classified and then allowed, so the tokenizer's understanding was the
// very thing that let them through.
func refuseUnheldState(stmt sqlparse.Statement, kind sqlparse.StmtKind) Decision {
	// USE is the one that has somewhere else to go. Naming the schema picker
	// matters more than explaining the connection pool, because the user
	// wanted a schema rather than a session.
	if stmt.Verb() == "USE" {
		return Decision{
			Verdict: Deny,
			Reason: "USE would apply to this statement alone; choose the schema instead, " +
				"and it travels with every statement",
		}
	}

	what := "session state"
	if kind == sqlparse.StmtTransaction {
		what = "a transaction"
	}
	return Decision{
		Verdict: Deny,
		Reason: fmt.Sprintf(
			"%s does not survive to the next statement: each one runs on its own connection from the pool",
			what),
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
			Verdict:    Deny,
			Reason:     reason + "; writes to production are locked",
			Unlockable: true,
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
