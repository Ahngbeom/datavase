package sqlparse

import "testing"

// The formatter upper-cases what this accepts. A word that can be a bare
// table name on either server must be refused, or a reformat could point a
// query at a different table on a case-sensitive file system.
func TestOnlyWordsReservedOnBothServersAreReserved(t *testing.T) {
	for _, w := range []string{"select", "From", "ORDER"} {
		if !IsReserved(w) {
			t.Errorf("IsReserved(%q) = false", w)
		}
	}
	for _, w := range []string{"users", "status", "end", "rank", "date"} {
		if IsReserved(w) {
			t.Errorf("IsReserved(%q) = true, but it can be a table name", w)
		}
	}
}
