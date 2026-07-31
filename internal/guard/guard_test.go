package guard

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

func policy(env config.Env) Policy {
	return Policy{Env: env, AutoLimit: 1000}
}

func decide(t *testing.T, sql string, p Policy) Decision {
	t.Helper()
	return Evaluate(sqlparse.Parse(sql), p)
}

func TestReadsAreAllowedEverywhere(t *testing.T) {
	reads := []string{
		"SELECT id FROM users LIMIT 10",
		"SHOW TABLES",
		"DESCRIBE users",
		"EXPLAIN SELECT 1",
	}

	for _, env := range []config.Env{config.EnvProd, config.EnvStage, config.EnvDev} {
		for _, sql := range reads {
			t.Run(string(env)+"/"+sql, func(t *testing.T) {
				got := decide(t, sql, policy(env))
				if got.Verdict != Allow {
					t.Errorf("Evaluate(%q) verdict = %v (%s), want Allow", sql, got.Verdict, got.Reason)
				}
			})
		}
	}
}

// Each statement runs on its own connection out of the pool, so a
// transaction cannot span two of them: BEGIN would open one on a connection
// that is handed straight back, and the ROLLBACK meant to undo the damage
// would find nothing to undo while reporting success. Refusing is the only
// honest answer until a transaction holds one connection for its lifetime.
func TestTransactionControlIsRefusedRatherThanSilentlyDiscarded(t *testing.T) {
	control := []string{
		"BEGIN",
		"START TRANSACTION",
		"COMMIT",
		"ROLLBACK",
		"SAVEPOINT sp1",
		"RELEASE SAVEPOINT sp1",
	}

	for _, env := range []config.Env{config.EnvProd, config.EnvStage, config.EnvDev} {
		for _, sql := range control {
			t.Run(string(env)+"/"+sql, func(t *testing.T) {
				got := decide(t, sql, policy(env))
				if got.Verdict != Deny {
					t.Errorf("Evaluate(%q) verdict = %v (%s), want Deny", sql, got.Verdict, got.Reason)
				}
				if !strings.Contains(got.Reason, "connection") {
					t.Errorf("reason = %q, want it to explain that the connection is not held", got.Reason)
				}
			})
		}
	}
}

// The same trap, one class wider: "SET SESSION sql_mode = ..." is accepted by
// the server and thrown away with the connection, so the next statement runs
// under the old mode while the user believes they changed it.
func TestSessionStateIsRefusedRatherThanSilentlyDiscarded(t *testing.T) {
	session := []string{
		"SET autocommit = 0",
		"SET SESSION sql_mode = 'STRICT_ALL_TABLES'",
		"SET @x = 1",
		"LOCK TABLES t WRITE",
		"UNLOCK TABLES",
	}

	for _, env := range []config.Env{config.EnvProd, config.EnvStage, config.EnvDev} {
		for _, sql := range session {
			t.Run(string(env)+"/"+sql, func(t *testing.T) {
				if got := decide(t, sql, policy(env)); got.Verdict != Deny {
					t.Errorf("Evaluate(%q) verdict = %v (%s), want Deny", sql, got.Verdict, got.Reason)
				}
			})
		}
	}
}

// USE is refused for the same reason, but it is the one session statement
// with somewhere else to go, so its refusal has to say so rather than leaving
// the user to guess that the schema picker exists.
func TestUseIsRefusedAndPointsAtTheSchemaPicker(t *testing.T) {
	got := decide(t, "USE app_db", policy(config.EnvDev))

	if got.Verdict != Deny {
		t.Fatalf("verdict = %v (%s), want Deny", got.Verdict, got.Reason)
	}
	if !strings.Contains(got.Reason, "schema") {
		t.Errorf("reason = %q, want it to send the user to the schema picker", got.Reason)
	}
}

// Writing to production must not be possible by accident; it takes an
// explicit session-level opt-in, and even then a confirmation.
func TestProductionWritesAreDeniedUntilWritesAreEnabled(t *testing.T) {
	writes := []string{
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET x = 1 WHERE id = 1",
		"DELETE FROM t WHERE id = 1",
	}

	for _, sql := range writes {
		t.Run(sql, func(t *testing.T) {
			locked := decide(t, sql, policy(config.EnvProd))
			if locked.Verdict != Deny {
				t.Errorf("with writes locked: verdict = %v, want Deny", locked.Verdict)
			}
			if !locked.Unlockable {
				t.Errorf("Unlockable = false; the caller has no way to offer the way past")
			}

			p := policy(config.EnvProd)
			p.WritesEnabled = true
			unlocked := Evaluate(sqlparse.Parse(sql), p)
			if unlocked.Verdict != Confirm {
				t.Errorf("with writes enabled: verdict = %v, want Confirm", unlocked.Verdict)
			}
		})
	}
}

// Unlockable is what the dialog reads to decide whether to offer a way past,
// so a refusal that the unlock would not actually lift must never carry it.
// Offering an escape hatch that does nothing is how a refusal stops being
// believed.
func TestRefusalsTheUnlockCannotLiftAreNotAdvertisedAsUnlockable(t *testing.T) {
	p := policy(config.EnvProd)
	p.WritesEnabled = true

	for _, sql := range []string{
		"DELETE FROM t",
		"UPDATE t SET x = 1",
		"DROP TABLE users",
		"TRUNCATE TABLE users",
		"ALTER TABLE users DROP COLUMN email",
		"GRANT ALL ON *.* TO u",
		"BEGIN",
		"SET autocommit = 0",
	} {
		t.Run(sql, func(t *testing.T) {
			got := Evaluate(sqlparse.Parse(sql), p)
			if got.Verdict != Deny {
				t.Fatalf("verdict = %v, want Deny", got.Verdict)
			}
			if got.Unlockable {
				t.Errorf("Unlockable = true, but enabling writes does not lift this refusal")
			}
		})
	}
}

func TestNonProductionWritesOnlyNeedConfirmation(t *testing.T) {
	for _, env := range []config.Env{config.EnvStage, config.EnvDev} {
		t.Run(string(env), func(t *testing.T) {
			got := decide(t, "UPDATE t SET x = 1 WHERE id = 1", policy(env))
			if got.Verdict != Confirm {
				t.Errorf("verdict = %v (%s), want Confirm", got.Verdict, got.Reason)
			}
		})
	}
}

// An unbounded UPDATE or DELETE rewrites the whole table. In production it
// is refused outright: no confirmation, no :write escape hatch.
func TestUnboundedWritesAreDeniedInProductionEvenWithWritesEnabled(t *testing.T) {
	unbounded := []string{
		"DELETE FROM t",
		"UPDATE t SET x = 1",
		"UPDATE t SET x = (SELECT y FROM u WHERE u.id = 1)",
	}

	p := policy(config.EnvProd)
	p.WritesEnabled = true

	for _, sql := range unbounded {
		t.Run(sql, func(t *testing.T) {
			got := Evaluate(sqlparse.Parse(sql), p)
			if got.Verdict != Deny {
				t.Errorf("verdict = %v, want Deny", got.Verdict)
			}
			if !strings.Contains(strings.ToUpper(got.Reason), "WHERE") {
				t.Errorf("reason = %q, want it to explain the missing WHERE", got.Reason)
			}
		})
	}
}

// Outside production the same statement is possible but deliberate: the user
// has to type the table name back.
func TestUnboundedWritesRequireTypedConfirmationOutsideProduction(t *testing.T) {
	got := decide(t, "DELETE FROM t", policy(config.EnvDev))

	if got.Verdict != Confirm {
		t.Fatalf("verdict = %v (%s), want Confirm", got.Verdict, got.Reason)
	}
	if got.TypeToConfirm == "" {
		t.Error("TypeToConfirm is empty, want a phrase the user must type")
	}
}

func TestDestructiveDDLIsDeniedInProduction(t *testing.T) {
	destructive := []string{
		"DROP TABLE users",
		"DROP DATABASE app",
		"TRUNCATE TABLE users",
		"ALTER TABLE users DROP COLUMN email",
		"CREATE TABLE t (id INT)",
	}

	p := policy(config.EnvProd)
	p.WritesEnabled = true

	for _, sql := range destructive {
		t.Run(sql, func(t *testing.T) {
			if got := Evaluate(sqlparse.Parse(sql), p); got.Verdict != Deny {
				t.Errorf("verdict = %v, want Deny", got.Verdict)
			}
		})
	}
}

func TestDestructiveDDLRequiresTypedConfirmationOutsideProduction(t *testing.T) {
	for _, sql := range []string{"DROP TABLE users", "TRUNCATE TABLE users"} {
		t.Run(sql, func(t *testing.T) {
			got := decide(t, sql, policy(config.EnvDev))
			if got.Verdict != Confirm {
				t.Fatalf("verdict = %v (%s), want Confirm", got.Verdict, got.Reason)
			}
			if got.TypeToConfirm == "" {
				t.Error("TypeToConfirm is empty, want a phrase the user must type")
			}
		})
	}
}

// The tokenizer is not a full parser, so anything it cannot classify is
// refused in production rather than assumed harmless.
func TestUnrecognisedStatementsFailClosed(t *testing.T) {
	const sql = "GRANT ALL PRIVILEGES ON *.* TO 'x'@'%'"

	if got := decide(t, sql, policy(config.EnvProd)); got.Verdict != Deny {
		t.Errorf("prod verdict = %v, want Deny", got.Verdict)
	}
	if got := decide(t, sql, policy(config.EnvDev)); got.Verdict != Confirm {
		t.Errorf("dev verdict = %v, want Confirm", got.Verdict)
	}
}

// A statement hidden in an executable comment must be judged by what the
// server will run, not by what it looks like.
func TestExecutableCommentsAreJudgedByTheirContents(t *testing.T) {
	got := decide(t, "/*!40001 DELETE FROM users */", policy(config.EnvProd))

	if got.Verdict != Deny {
		t.Errorf("verdict = %v (%s), want Deny", got.Verdict, got.Reason)
	}
}

func TestEmptyStatementIsRejected(t *testing.T) {
	for _, sql := range []string{"", "   ", "-- just a note"} {
		t.Run(sql, func(t *testing.T) {
			if got := decide(t, sql, policy(config.EnvDev)); got.Verdict != Deny {
				t.Errorf("verdict = %v, want Deny", got.Verdict)
			}
		})
	}
}

func TestAutoLimitIsProposedForUnboundedSelects(t *testing.T) {
	got := decide(t, "SELECT * FROM users", policy(config.EnvProd))

	if got.Verdict != Allow {
		t.Fatalf("verdict = %v (%s), want Allow", got.Verdict, got.Reason)
	}
	if got.InjectLimit != 1000 {
		t.Errorf("InjectLimit = %d, want 1000", got.InjectLimit)
	}
}

func TestAutoLimitLeavesExplicitLimitsAlone(t *testing.T) {
	got := decide(t, "SELECT * FROM users LIMIT 5", policy(config.EnvProd))

	if got.InjectLimit != 0 {
		t.Errorf("InjectLimit = %d, want 0; an explicit LIMIT must win", got.InjectLimit)
	}
}

func TestAutoLimitCanBeDisabled(t *testing.T) {
	p := policy(config.EnvProd)
	p.AutoLimit = 0

	if got := decide(t, "SELECT * FROM users", p); got.InjectLimit != 0 {
		t.Errorf("InjectLimit = %d, want 0 when AutoLimit is off", got.InjectLimit)
	}
}

// SHOW and friends do not accept a trailing LIMIT the way SELECT does.
func TestAutoLimitIsNotAppliedToNonSelectReads(t *testing.T) {
	for _, sql := range []string{"SHOW TABLES", "DESCRIBE users", "USE app_db"} {
		t.Run(sql, func(t *testing.T) {
			if got := decide(t, sql, policy(config.EnvProd)); got.InjectLimit != 0 {
				t.Errorf("InjectLimit = %d, want 0", got.InjectLimit)
			}
		})
	}
}

// Every decision has to explain itself: the reason is what the status bar
// and the confirmation dialog show.
func TestEveryNonAllowDecisionCarriesAReason(t *testing.T) {
	samples := []struct {
		sql string
		env config.Env
	}{
		{"DELETE FROM t", config.EnvProd},
		{"DELETE FROM t", config.EnvDev},
		{"DROP TABLE t", config.EnvProd},
		{"UPDATE t SET x = 1 WHERE id = 1", config.EnvStage},
		{"GRANT ALL ON *.* TO u", config.EnvProd},
		{"", config.EnvDev},
	}

	for _, s := range samples {
		t.Run(string(s.env)+"/"+s.sql, func(t *testing.T) {
			got := decide(t, s.sql, policy(s.env))
			if got.Verdict == Allow {
				t.Skip("allowed; nothing to explain")
			}
			if strings.TrimSpace(got.Reason) == "" {
				t.Error("Reason is empty")
			}
		})
	}
}
