// Package complete turns a caret position into completion candidates.
//
// The engine is pure logic over a Catalog lookup, so the interesting part —
// which names belong at which position — is tested without a database or a
// terminal.
package complete

import (
	"context"
	"sort"
	"strings"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// MaxCandidates bounds the popup. A schema with thousands of tables would
// otherwise produce a list nobody can navigate.
const MaxCandidates = 200

// Catalog is the metadata source. It is an interface so the engine can be
// tested without SQLite, and so a cold cache is just an empty answer.
type Catalog interface {
	Schemas(ctx context.Context, datasource string) ([]string, error)
	Tables(ctx context.Context, datasource, schema string) ([]string, error)
	Columns(ctx context.Context, datasource, schema, table string) ([]string, error)
}

// Kind classifies a candidate, so the popup can label it.
type Kind int

const (
	KindTable Kind = iota
	KindColumn
	KindSchema
	KindKeyword
)

func (k Kind) String() string {
	switch k {
	case KindTable:
		return "table"
	case KindColumn:
		return "column"
	case KindSchema:
		return "schema"
	case KindKeyword:
		return "keyword"
	default:
		return "?"
	}
}

// Candidate is one suggestion, together with the text it replaces.
type Candidate struct {
	Text   string
	Kind   Kind
	Detail string
	// ReplaceFrom and ReplaceTo delimit what accepting this candidate
	// overwrites, so the editor does not have to work it out again.
	ReplaceFrom, ReplaceTo int
}

// Engine produces candidates for one datasource.
type Engine struct {
	catalog       Catalog
	datasource    string
	defaultSchema string
}

// New returns an engine reading from catalog.
func New(catalog Catalog, datasource, defaultSchema string) *Engine {
	return &Engine{catalog: catalog, datasource: datasource, defaultSchema: defaultSchema}
}

// SetSchema changes the schema unqualified names resolve against, which
// follows the tree selection and any USE statement.
func (e *Engine) SetSchema(schema string) { e.defaultSchema = schema }

// Suggest returns candidates for the caret position in sql.
//
// A catalog failure yields no candidates rather than an error: completion
// runs on a keystroke, and a cold or locked cache must never interrupt
// typing.
func (e *Engine) Suggest(ctx context.Context, sql string, offset int) ([]Candidate, error) {
	position := sqlparse.CompletionAt(sql, offset)

	var candidates []Candidate
	switch position.Kind {
	case sqlparse.CompleteTable:
		candidates = e.tables(ctx, e.defaultSchema)
	case sqlparse.CompleteColumn:
		candidates = e.columnsInScope(ctx, position.Tables)
	case sqlparse.CompleteQualified:
		candidates = e.qualified(ctx, position)
	default:
		return nil, nil
	}

	// Keywords are offered alongside names; they are what completes an
	// unfinished clause, which names cannot.
	if position.Kind != sqlparse.CompleteQualified {
		candidates = append(candidates, keywordCandidates()...)
	}

	return rank(candidates, position), nil
}

// qualified resolves "name." — an alias or table first, a schema second.
//
// The parser cannot tell the two apart: in "FROM log_db." the name sits where
// a table belongs, and only the catalog knows whether such a table exists.
// Tables are tried first because the more local reading is usually the
// intended one; an empty answer falls through to the schema reading.
func (e *Engine) qualified(ctx context.Context, position sqlparse.CompletionContext) []Candidate {
	if ref, ok := sqlparse.ResolveQualifier(position.Tables, position.Qualifier); ok && !ref.Derived {
		if columns := e.columns(ctx, e.schemaOf(ref), ref.Name); len(columns) > 0 {
			return columns
		}
	}

	for _, schema := range e.schemas(ctx) {
		if strings.EqualFold(schema, position.Qualifier) {
			return e.tables(ctx, schema)
		}
	}
	return nil
}

// columnsInScope gathers the columns of every table the statement mentions.
func (e *Engine) columnsInScope(ctx context.Context, refs []sqlparse.TableRef) []Candidate {
	var (
		out  []Candidate
		seen = make(map[string]bool)
	)

	for _, ref := range refs {
		if ref.Derived || ref.Name == "" {
			continue
		}
		for _, c := range e.columns(ctx, e.schemaOf(ref), ref.Name) {
			// The same column name in two joined tables is one entry: the
			// list is for typing, not for describing the schema.
			key := strings.ToLower(c.Text)
			if seen[key] {
				continue
			}
			seen[key] = true

			c.Detail = ref.Name
			out = append(out, c)
		}
	}
	return out
}

func (e *Engine) schemaOf(ref sqlparse.TableRef) string {
	if ref.Schema != "" {
		return ref.Schema
	}
	return e.defaultSchema
}

func (e *Engine) schemas(ctx context.Context) []string {
	names, err := e.catalog.Schemas(ctx, e.datasource)
	if err != nil {
		return nil
	}
	return names
}

func (e *Engine) tables(ctx context.Context, schema string) []Candidate {
	names, err := e.catalog.Tables(ctx, e.datasource, schema)
	if err != nil {
		return nil
	}

	out := make([]Candidate, 0, len(names))
	for _, name := range names {
		out = append(out, Candidate{Text: name, Kind: KindTable, Detail: schema})
	}
	return out
}

func (e *Engine) columns(ctx context.Context, schema, table string) []Candidate {
	names, err := e.catalog.Columns(ctx, e.datasource, schema, table)
	if err != nil {
		return nil
	}

	out := make([]Candidate, 0, len(names))
	for _, name := range names {
		out = append(out, Candidate{Text: name, Kind: KindColumn, Detail: table})
	}
	return out
}

// rank filters by the typed prefix, orders the survivors and caps the list.
//
// Ordering puts prefix matches first: someone typing "user" is far more
// likely to want "users" than "my_user", and a list that buries the obvious
// answer is worse than no list.
func rank(candidates []Candidate, position sqlparse.CompletionContext) []Candidate {
	prefix := strings.ToLower(position.Prefix)

	matched := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		lower := strings.ToLower(c.Text)
		if prefix != "" && !strings.Contains(lower, prefix) {
			continue
		}
		c.ReplaceFrom = position.ReplaceFrom
		c.ReplaceTo = position.ReplaceTo
		matched = append(matched, c)
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return rankOf(matched[i], prefix) < rankOf(matched[j], prefix)
	})

	if len(matched) > MaxCandidates {
		matched = matched[:MaxCandidates]
	}
	return matched
}

// rankOf scores a candidate: lower sorts earlier.
func rankOf(c Candidate, prefix string) int {
	score := 0
	if prefix != "" && !strings.HasPrefix(strings.ToLower(c.Text), prefix) {
		score += 100 // a match further inside the name
	}
	// Names before keywords: the schema is the specific knowledge, keywords
	// are always available and rarely what a partial word is reaching for.
	if c.Kind == KindKeyword {
		score += 10
	}
	return score
}

// keywords are offered where a clause could continue.
var keywords = []string{
	"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "HAVING", "LIMIT",
	"INNER JOIN", "LEFT JOIN", "RIGHT JOIN", "JOIN", "ON", "AS",
	"AND", "OR", "NOT", "IN", "IS NULL", "IS NOT NULL", "LIKE", "BETWEEN",
	"COUNT(", "SUM(", "AVG(", "MIN(", "MAX(", "DISTINCT",
	"INSERT INTO", "VALUES", "UPDATE", "SET", "DELETE FROM",
	"UNION", "UNION ALL", "CASE", "WHEN", "THEN", "ELSE", "END",
	"DESC", "ASC", "EXISTS", "COALESCE(", "IFNULL(",
}

func keywordCandidates() []Candidate {
	out := make([]Candidate, len(keywords))
	for i, kw := range keywords {
		out[i] = Candidate{Text: kw, Kind: KindKeyword}
	}
	return out
}
