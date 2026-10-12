package engine

import (
	"github.com/odvcencio/gotreesitter"
)

// gtsQuery/gtsQueryCursor have no-op Close methods: gotreesitter is pure Go
// and garbage-collected, so Query/QueryCursor hold no external resources to
// release; Close exists only to satisfy the engine interfaces shared with
// the CGO backend, whose Close calls do matter.
type gtsQuery struct {
	q *gotreesitter.Query
}

func (q *gtsQuery) Close() {}

type gtsQueryCursor struct {
	lang *gotreesitter.Language
}

func (c *gtsQueryCursor) Close() {}

func (c *gtsQueryCursor) Matches(query Query, root Node, source []byte) QueryMatches {
	q := query.(*gtsQuery).q
	r := root.(*gtsNode).n
	// Exec is bound to the cursor's own lang -- the language query was
	// compiled against, via gtsLanguage.NewQuery/NewQueryCursor sharing one
	// lang() value -- not derived from root, so a query executed against a
	// tree parsed with a different (but node-kind-name-compatible) grammar
	// yields no matches rather than misinterpreted symbol IDs, matching
	// go-tree-sitter's cross-grammar behavior (confirmed empirically).
	return &gtsQueryMatches{c: q.Exec(r, c.lang, source), lang: c.lang}
}

type gtsQueryMatches struct {
	c    *gotreesitter.QueryCursor
	lang *gotreesitter.Language
}

func (m *gtsQueryMatches) Next() *QueryMatch {
	match, ok := m.c.NextMatch()
	if !ok {
		return nil
	}
	captures := make([]QueryCapture, len(match.Captures))
	for i, c := range match.Captures {
		captures[i] = QueryCapture{
			Node:  &gtsNode{n: c.Node, lang: m.lang},
			Index: uint32(i),
		}
	}
	return &QueryMatch{Captures: captures}
}
