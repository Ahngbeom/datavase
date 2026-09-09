package sqlparse

import "strings"

// QuoteIdentifier renders a name as a backtick-quoted MySQL identifier.
//
// A backtick inside a name is escaped by doubling it, which is MySQL's own
// rule. Without this, a table called "we`ird" would end the quoted section
// early and turn the rest of the name into syntax.
func QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
