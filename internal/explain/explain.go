// Package explain turns a server's JSON query plan into something readable.
//
// It knows nothing about tview and nothing about a connection: bytes in, a
// tree and then lines out. What it deliberately does not know is the plan's
// schema.
//
// MySQL and MariaDB disagree about that schema, and both change it between
// versions — MariaDB nests a sort under "read_sorted_file" and a group under
// "temporary_table", MySQL under "ordering_operation" and "grouping_operation",
// and neither list is fixed. So the walk is generic: an object's scalar fields
// are that step's attributes and its object and array fields are its children.
// What appears on screen is then what the server said, rather than what this
// package assumed it would say.
package explain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Node is one step of a plan.
type Node struct {
	// Name is the key the server nested this step under — "query_block",
	// "filesort", "table" — or the table's own name where it has one, since
	// "table" says less than "dv_x_books".
	Name string
	// Attrs are the step's scalar fields, in the order they are worth reading
	// rather than the order they arrived.
	Attrs    []Attr
	Children []*Node
	// Warnings name what will make this step slow, in the words a DBA would
	// use rather than the server's.
	Warnings []string
}

// Attr is one scalar field of a step.
type Attr struct {
	Key   string
	Value string
}

// Attr returns a field's value, or "" when the step has no such field.
func (n *Node) Attr(key string) string {
	for _, a := range n.Attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

// Parse reads a plan from EXPLAIN FORMAT=JSON.
func Parse(data []byte) (*Node, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("reading the plan: %w", err)
	}

	// A plan without a query block is not a plan. Rendering one anyway would
	// put an empty tree on screen, which reads as a query with no steps rather
	// than as the failure it is.
	root, ok := raw["query_block"]
	if !ok {
		return nil, fmt.Errorf("no query_block in the plan")
	}

	block, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the query_block is not an object")
	}
	return build("query_block", block), nil
}

// build turns one JSON object into a step and its descendants.
func build(name string, obj map[string]any) *Node {
	n := &Node{Name: name}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	// Sorted so that the same plan renders the same way twice; Go's map order
	// is deliberately not.
	sort.Strings(keys)

	for _, key := range keys {
		switch value := obj[key].(type) {
		case map[string]any:
			n.Children = append(n.Children, build(key, value))

		case []any:
			n.appendList(key, value)

		default:
			if text := scalar(value); text != "" {
				n.Attrs = append(n.Attrs, Attr{Key: key, Value: text})
			}
		}
	}

	sortAttrs(n.Attrs)
	n.label()
	n.warn()
	return n
}

// appendList handles a field holding an array.
//
// Arrays are two different things in a plan and telling them apart is the one
// place a shape is assumed: a list of objects is a list of steps, and a list
// of scalars — possible_keys, used_key_parts, ref — is one value.
func (n *Node) appendList(key string, items []any) {
	var scalars []string

	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			// A step in a nested_loop arrives wrapped in a one-key object, so
			// the wrapper is skipped and its content named after the key it
			// held. Keeping it would put an unnamed level between every pair
			// of real ones.
			if len(obj) == 1 {
				for inner, content := range obj {
					if nested, isObj := content.(map[string]any); isObj {
						n.Children = append(n.Children, build(inner, nested))
						continue
					}
				}
				if onlyObject(obj) {
					continue
				}
			}
			n.Children = append(n.Children, build(key, obj))
			continue
		}
		if text := scalar(item); text != "" {
			scalars = append(scalars, text)
		}
	}

	if len(scalars) > 0 {
		n.Attrs = append(n.Attrs, Attr{Key: key, Value: strings.Join(scalars, ", ")})
	}
}

func onlyObject(obj map[string]any) bool {
	for _, v := range obj {
		if _, ok := v.(map[string]any); ok {
			return true
		}
	}
	return false
}

// label renames a table step after the table it reads.
//
// "table" is the same word for every one of them, and a plan whose steps are
// all called "table" is a plan you have to read the attributes of before you
// know what you are looking at.
func (n *Node) label() {
	if name := n.Attr("table_name"); name != "" {
		n.Name = name
	}
}

// warn flags what makes a step expensive.
//
// These are the ones a plan is usually read to find, and they are named as a
// reader would say them rather than as the server spells them. Every one of
// them has to be something the reader could act on: a warning that names a
// cost nothing can remove is how a reader learns to skip the warnings.
func (n *Node) warn() {
	// A full scan on an internal table — <union1,2>, <derived2>,
	// <subquery3> — is the server reading back what it has just written, and
	// it is access_type ALL every single time. There is no index to add.
	if n.Attr("access_type") == "ALL" && !strings.HasPrefix(n.Name, "<") {
		n.Warnings = append(n.Warnings, "full scan")
	}

	switch n.Name {
	case "filesort":
		n.Warnings = append(n.Warnings, "filesort")
	case "temporary_table", "duplicates_removal":
		n.Warnings = append(n.Warnings, "temporary table")
	}

	// MySQL says it with a flag on the step that does the ordering, where
	// MariaDB gives the sort a node of its own. read_sorted_file is not
	// listed above for the same reason: it is MariaDB's wrapper around a
	// filesort node that is already going to be flagged.
	if n.Attr("using_filesort") == "true" {
		n.Warnings = append(n.Warnings, "filesort")
	}
	if n.Attr("using_temporary_table") == "true" {
		n.Warnings = append(n.Warnings, "temporary table")
	}
}

// attrOrder is the order the fields are worth reading in: what the step does,
// then how much it costs, then the detail.
//
// Anything not named here follows, alphabetically. A field this list has never
// heard of is a field a newer server added, and dropping it would make the
// plan quietly less true than the one the server sent.
var attrOrder = map[string]int{
	"access_type":    1,
	"key":            2,
	"key_length":     3,
	"ref":            4,
	"rows":           5,
	"r_rows":         6,
	"filtered":       7,
	"r_filtered":     8,
	"cost":           9,
	"loops":          10,
	"r_loops":        11,
	"select_id":      12,
	"sort_key":       13,
	"possible_keys":  14,
	"used_key_parts": 15,
}

func sortAttrs(attrs []Attr) {
	sort.SliceStable(attrs, func(i, j int) bool {
		ri, rj := rank(attrs[i].Key), rank(attrs[j].Key)
		if ri != rj {
			return ri < rj
		}
		return attrs[i].Key < attrs[j].Key
	})
}

func rank(key string) int {
	if r, ok := attrOrder[key]; ok {
		return r
	}
	return len(attrOrder) + 1
}

// scalar renders a JSON value, dropping the ones that say nothing.
func scalar(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		if value {
			return "true"
		}
		return "false"
	case float64:
		// %g rather than a fixed precision: costs arrive as 0.018438808 and
		// row counts as 4, and neither should be shown in the other's shape.
		return fmt.Sprintf("%g", value)
	default:
		return fmt.Sprint(value)
	}
}
