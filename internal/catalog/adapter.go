package catalog

import "context"

// NameSource adapts the cache to the name-only lookup the completion engine
// needs.
//
// The engine deals in identifiers, not in schema detail, so this is the whole
// surface it sees — which is what keeps the engine testable with a map.
type NameSource struct {
	cache *Cache
}

// Names returns a completion-friendly view of the cache.
func (c *Cache) Names() *NameSource { return &NameSource{cache: c} }

func (n *NameSource) Schemas(ctx context.Context, datasource string) ([]string, error) {
	return n.cache.Schemas(ctx, datasource)
}

func (n *NameSource) Tables(ctx context.Context, datasource, schema string) ([]string, error) {
	tables, err := n.cache.Tables(ctx, datasource, schema)
	if err != nil {
		return nil, err
	}

	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Name
	}
	return out, nil
}

func (n *NameSource) Columns(ctx context.Context, datasource, schema, table string) ([]string, error) {
	columns, err := n.cache.Columns(ctx, datasource, schema, table)
	if err != nil {
		return nil, err
	}

	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = c.Name
	}
	return out, nil
}
