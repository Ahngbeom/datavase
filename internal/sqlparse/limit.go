package sqlparse

import "strings"

// AppendLimit returns stmt's SQL with "LIMIT n" inserted, or the SQL
// unchanged when doing so would be unsafe.
//
// Insertion is by token position rather than string concatenation, because
// the two shapes that break naive appending are common:
//
//   - a trailing comment would swallow the clause
//   - a locking clause (FOR UPDATE, LOCK IN SHARE MODE) must follow LIMIT,
//     not precede it
//
// Shapes this cannot place the clause in confidently — SELECT ... INTO — are
// returned untouched. An over-large result set is an inconvenience; a query
// rewritten into invalid SQL destroys trust in the tool.
func AppendLimit(stmt Statement, n int) string {
	if n <= 0 || stmt.IsEmpty() || stmt.HasTopLevelLimit() {
		return stmt.SQL
	}
	if stmt.hasTopLevelKeyword("INTO") {
		return stmt.SQL
	}

	at, ok := stmt.limitInsertionPoint()
	if !ok {
		return stmt.SQL
	}

	base := stmt.Pos
	clause := " LIMIT " + itoa(n)

	// Trim on both sides of the seam so the clause lands with exactly one
	// space around it regardless of how the original was spaced.
	head := strings.TrimRight(stmt.SQL[:at-base], " \t")
	tail := strings.TrimLeft(stmt.SQL[at-base:], " \t")
	if tail == "" {
		return head + clause
	}
	return head + clause + " " + tail
}

// limitInsertionPoint returns the offset in the source where the clause
// belongs: before any top-level locking clause, and before any trailing
// comment, but after the last piece of executable text.
func (s Statement) limitInsertionPoint() (int, bool) {
	end := -1
	for _, tk := range s.Tokens {
		if tk.Kind == Comment {
			continue
		}
		if tk.Depth == 0 && startsLockingClause(tk) {
			return tk.Pos, true
		}
		end = tk.End
	}
	if end < 0 {
		return 0, false
	}
	return end, true
}

// startsLockingClause reports whether the token begins a clause that must
// come after LIMIT: FOR UPDATE, FOR SHARE, LOCK IN SHARE MODE.
func startsLockingClause(tk Token) bool {
	return tk.IsKeyword("FOR") || tk.IsKeyword("LOCK")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
