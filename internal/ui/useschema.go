package ui

import (
	"context"
	"fmt"
	"strings"
)

// showUseSchema offers the schemas of this server, filtered as the user types.
//
// The schema tree answers "what is on this server"; this answers "which one
// should the next unqualified query hit", which is a decision rather than a
// browse and so gets a decision's worth of ceremony.
func (a *App) showUseSchema() {
	if a.cache == nil {
		a.notice("the schema cache is unavailable")
		return
	}

	current := a.currentSchema()

	box := a.newSearchBox("schema: ", " choose a schema ", pageUseSchema, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		schemas, err := a.cache.Schemas(ctx, a.conn.DataSource().Name)
		if err != nil {
			return []searchItem{message("could not read the schemas", err.Error())}
		}

		items := make([]searchItem, 0, len(schemas))
		for _, s := range schemas {
			schema := s
			if !matchesSchema(schema, term) {
				continue
			}

			detail := "switch to this schema"
			if schema == current {
				// Marked rather than hidden: seeing which one is in force is
				// half the reason for opening this at all.
				detail = "current"
			}
			name := schema
			if schema == current {
				name += "  " + currentSchemaMarker
			}
			items = append(items, searchItem{
				primary:   name,
				secondary: detail,
				accept: func() {
					a.closeSearchBox(pageUseSchema)
					a.useSchema(schema)
				},
			})
		}

		if len(items) == 0 {
			if term == "" {
				return []searchItem{nothingHere("no schemas cached yet",
					"the schema tree is still loading")}
			}
			return []searchItem{noMatch("schema", term)}
		}
		return items
	})

	a.pages.AddPage(pageUseSchema, centred(box, 60, 20), true, true)
}

func matchesSchema(schema, term string) bool {
	term = strings.TrimSpace(term)
	if term == "" {
		return true
	}
	return strings.Contains(strings.ToLower(schema), strings.ToLower(term))
}

// useSchema makes a schema the one unqualified names resolve against.
//
// This is more than a label: the schema travels with every statement the
// editor runs, so the choice reaches the server rather than only the status
// bar. A picker that changed what the interface said but not where the query
// went would be worse than no picker.
func (a *App) useSchema(schema string) {
	a.setSchema(schema)
	a.notice(fmt.Sprintf("using %s", schema))
}
