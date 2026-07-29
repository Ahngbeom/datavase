package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/result"
)

// currentSchemaMarker flags the schema an unqualified query will hit.
// Nothing else on screen says which one that is.
const currentSchemaMarker = "●"

// rootLabel describes the tree's root: the server, not a schema.
//
// This exists because a datasource is often named after its main schema, and
// when both carry the same name the root reads as a schema with the real
// schemas nested beneath it. The host is what makes the root unmistakably a
// server.
//
// When the pane is too narrow the datasource name gives way rather than the
// host: a root showing only a name is exactly the state being fixed.
func rootLabel(ds *config.DataSource, width int) string {
	host := fmt.Sprintf("%s:%d", ds.Host, ds.Port)

	if width <= 0 {
		return result.EscapeTags(host)
	}

	const separator = " · "
	full := ds.Name + separator + host
	if utf8.RuneCountInString(full) <= width {
		return result.EscapeTags(full)
	}

	// Give the name whatever is left once the host and separator are placed.
	room := width - utf8.RuneCountInString(host) - utf8.RuneCountInString(separator)
	if room < 3 {
		// Not even a stub of the name fits; the host alone still identifies
		// the server, which is the point of the label.
		return result.EscapeTags(result.Truncate(host, width))
	}
	return result.EscapeTags(result.Truncate(ds.Name, room) + separator + host)
}

// schemaLabel renders one schema, marking the current one.
func schemaLabel(name, current string) string {
	label := result.EscapeTags(name)
	if current != "" && strings.EqualFold(name, current) {
		return label + "  " + currentSchemaMarker
	}
	return label
}
