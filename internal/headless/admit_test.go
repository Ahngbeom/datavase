package headless

import (
	"errors"
	"strings"
	"testing"
)

func TestPlainReadsAreAdmitted(t *testing.T) {
	for _, sql := range []string{
		"SELECT id, email FROM users WHERE id = 1",
		"select count(*) from orders;",
		"WITH recent AS (SELECT * FROM orders) SELECT COUNT(*) FROM recent",
		"TABLE users",
		"SHOW TABLES",
		"SHOW CREATE TABLE users",
		"DESCRIBE users",
		"DESC users",
		"EXPLAIN SELECT * FROM users",
		"EXPLAIN FORMAT=JSON SELECT * FROM users",
		// String functions that share a name with a write verb.
		"SELECT REPLACE(name, 'a', 'b') FROM users",
		"SELECT INSERT(name, 1, 2, 'xy') FROM users",
		// Words that only look dangerous inside a literal, a quoted name or a comment.
		"SELECT 'DELETE FROM users; SLEEP(5)' AS note",
		"SELECT `update` FROM audit",
		"SELECT 1 /* INTO OUTFILE */",
		"SELECT 1 -- FOR UPDATE",
		"SELECT '/*! DELETE */' AS s",
		// An optimizer hint is not an executable comment.
		"SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM users",
	} {
		if _, err := Admit(sql); err != nil {
			t.Errorf("Admit(%q) = %v, want it admitted", sql, err)
		}
	}
}

func TestAdmitReturnsTheStatementThatWillRun(t *testing.T) {
	stmt, err := Admit("  SELECT 1 ;\n")
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if stmt.SQL != "SELECT 1" {
		t.Errorf("stmt.SQL = %q, want %q", stmt.SQL, "SELECT 1")
	}
}

// Every refusal must happen before anything is sent: a program reading the
// output cannot take back a statement the server already ran.
func TestAnythingButAPlainReadIsRefusedBeforeItRuns(t *testing.T) {
	for _, tc := range []struct {
		sql, why string
	}{
		{"", "statement"},
		{"-- only a comment", "statement"},
		{"SELECT 1; SELECT 2", "one statement"},
		{"SELECT 1; DELETE FROM users", "one statement"},

		{"UPDATE users SET email = NULL", "UPDATE"},
		{"DELETE FROM users", "DELETE"},
		{"INSERT INTO users (id) VALUES (1)", "INSERT"},
		{"TRUNCATE users", "TRUNCATE"},
		{"DROP TABLE users", "DROP"},
		{"CREATE TABLE t (id INT)", "DDL"},
		{"SET SESSION TRANSACTION READ WRITE", "session"},
		{"USE other_schema", "session"},
		{"START TRANSACTION", "transaction"},
		{"CALL purge_everything()", "unrecognised"},
		{"CHECK TABLE users", "CHECK"},
		{"ANALYZE TABLE users", "ANALYZE"},

		// The wrappers that run what they wrap.
		{"ANALYZE FORMAT=JSON DELETE FROM orders", "DELETE"},
		{"EXPLAIN ANALYZE SELECT * FROM users", "ANALYZE"},

		// Executable comments: the server runs what the tokenizer would
		// otherwise have to be trusted to have seen.
		{"SELECT 1 /*! , SLEEP(5) */", "executable comment"},
		{"SELECT * FROM users /*!50000 FOR UPDATE */", "executable comment"},

		// A CTE in front of a write is a write.
		{"WITH x AS (SELECT 1) UPDATE users SET email = NULL", "write"},
		{"WITH x AS (SELECT id FROM users) DELETE FROM users WHERE id IN (SELECT id FROM x)", "write"},

		{"SELECT * FROM users INTO OUTFILE '/tmp/users.csv'", "INTO"},
		{"SELECT * INTO DUMPFILE '/tmp/x' FROM users", "INTO"},
		{"SELECT id INTO @kept FROM users LIMIT 1", "INTO"},

		{"SELECT * FROM users FOR UPDATE", "lock"},
		{"SELECT * FROM users FOR SHARE", "lock"},
		{"SELECT * FROM users LOCK IN SHARE MODE", "lock"},
		{"SELECT GET_LOCK('k', 10)", "GET_LOCK"},

		{"SELECT SLEEP(10)", "SLEEP"},
		{"SELECT BENCHMARK(100000000, MD5('x'))", "BENCHMARK"},
		{"SELECT LOAD_FILE('/etc/passwd')", "LOAD_FILE"},
		{"SELECT id FROM users WHERE (SELECT sleep (3)) = 0", "SLEEP"},
	} {
		_, err := Admit(tc.sql)

		var refused *Refusal
		if !errors.As(err, &refused) {
			t.Errorf("Admit(%q) = %v, want a *Refusal", tc.sql, err)
			continue
		}
		if !strings.Contains(strings.ToLower(refused.Error()), strings.ToLower(tc.why)) {
			t.Errorf("Admit(%q) said %q, want it to name %q", tc.sql, refused, tc.why)
		}
	}
}
