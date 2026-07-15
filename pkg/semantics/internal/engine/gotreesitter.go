// Adapts github.com/odvcencio/gotreesitter (a from-scratch, CGO-free
// reimplementation of the Tree-sitter runtime) to the engine interfaces.
package engine

import (
	"fmt"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// GoTreeSitterLanguage adapts a gotreesitter grammar, looked up by its
// registered name (e.g. "go", "typescript", "tsx"), into a Language. name
// must be registered in github.com/odvcencio/gotreesitter/grammars --
// typically via a matching grammar_subset_<name> build tag (see
// mise.toml's wasm-build task). An unregistered name is a
// build configuration error, not a runtime condition callers should need to
// handle, so this panics rather than returning an error: it is only ever
// called from package-init-time languageRegistry construction with names
// this package controls.
func GoTreeSitterLanguage(name string) Language {
	entry := grammars.DetectLanguageByName(name)
	if entry == nil {
		panic(fmt.Sprintf("engine: no gotreesitter grammar registered for %q (missing a grammar_subset_%s build tag?)", name, name))
	}
	if name == "typescript" || name == "tsx" {
		// gotreesitter's plain parse path misparses plain-identifier default
		// parameters (e.g. `function f(x = 1) {}`) and array-destructuring
		// defaults (e.g. `const [a = 2] = z;`) as syntax errors. WantsForest
		// is gotreesitter's own documented opt-in (see gotreesitter's
		// language.go) that routes parsing through its GSS-forest GLR path,
		// which handles these shapes correctly and falls back to the
		// existing parser automatically on any forest failure or error node,
		// so it's a strict improvement with no regression risk.
		entry.Language().WantsForest = true
	}
	return &gtsLanguage{entry: entry}
}

type gtsLanguage struct {
	entry *grammars.LangEntry
}

func (l *gtsLanguage) lang() *gotreesitter.Language { return l.entry.Language() }

func (l *gtsLanguage) NewParser() (Parser, error) {
	return &gtsParser{entry: l.entry, lang: l.lang()}, nil
}

func (l *gtsLanguage) NewQuery(source string) (Query, error) {
	q, err := gotreesitter.NewQuery(source, l.lang())
	if err != nil {
		return nil, err
	}
	return &gtsQuery{q: q}, nil
}

func (l *gtsLanguage) NewQueryCursor() QueryCursor {
	return &gtsQueryCursor{lang: l.lang()}
}

// gtsParser mirrors grammars.ParseFile's own dispatch: real grammars that
// need a custom lexer (currently just Go, whose lexing doesn't fit
// Tree-sitter's external-scanner model) register a TokenSourceFactory on
// their LangEntry; grammars satisfied by the external-scanner-augmented DFA
// lexer (TypeScript, TSX) leave it nil and use the plain Parse path. Either
// way, plain Parse/ParseWithTokenSource (never the *Strict variants) always
// return a tree with a nil error, even for malformed input -- confirmed
// empirically -- so HasError() on the resulting root is the only signal
// callers need, matching go-tree-sitter's contract.
type gtsParser struct {
	entry *grammars.LangEntry
	lang  *gotreesitter.Language
}

func (p *gtsParser) Parse(content []byte) (Tree, error) {
	parser := gotreesitter.NewParser(p.lang)

	var tree *gotreesitter.Tree
	var err error
	if p.entry.TokenSourceFactory != nil {
		ts := p.entry.TokenSourceFactory(content, p.lang)
		tree, err = parser.ParseWithTokenSource(content, ts)
	} else {
		tree, err = parser.Parse(content)
	}
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, nil
	}
	return &gtsTree{t: tree, lang: p.lang}, nil
}

type gtsTree struct {
	t    *gotreesitter.Tree
	lang *gotreesitter.Language
}

func (t *gtsTree) RootNode() Node {
	root := t.t.RootNode()
	if root == nil {
		return nil
	}
	return &gtsNode{n: root, lang: t.lang}
}

func (t *gtsTree) Close() { t.t.Release() }

// gtsNode carries lang alongside its *gotreesitter.Node because, unlike
// go-tree-sitter's Node, gotreesitter's Type/ChildByFieldName resolve node
// kind names and field lookups through an explicit *Language argument
// rather than a language the node is intrinsically bound to.
type gtsNode struct {
	n    *gotreesitter.Node
	lang *gotreesitter.Language
}

func (n *gtsNode) Kind() string    { return n.n.Type(n.lang) }
func (n *gtsNode) HasError() bool  { return n.n.HasError() }
func (n *gtsNode) IsError() bool   { return n.n.IsError() }
func (n *gtsNode) IsMissing() bool { return n.n.IsMissing() }
func (n *gtsNode) ChildCount() int { return n.n.ChildCount() }
func (n *gtsNode) StartByte() uint { return uint(n.n.StartByte()) }
func (n *gtsNode) EndByte() uint   { return uint(n.n.EndByte()) }

func (n *gtsNode) Child(i int) Node {
	c := n.n.Child(i)
	if c == nil {
		return nil
	}
	return &gtsNode{n: c, lang: n.lang}
}

func (n *gtsNode) ChildByFieldName(name string) Node {
	c := n.n.ChildByFieldName(name, n.lang)
	if c == nil {
		return nil
	}
	return &gtsNode{n: c, lang: n.lang}
}

func (n *gtsNode) Parent() Node {
	p := n.n.Parent()
	if p == nil {
		return nil
	}
	return &gtsNode{n: p, lang: n.lang}
}

func (n *gtsNode) StartPoint() (row, col uint) {
	p := n.n.StartPoint()
	return uint(p.Row), uint(p.Column)
}

func (n *gtsNode) EndPoint() (row, col uint) {
	p := n.n.EndPoint()
	return uint(p.Row), uint(p.Column)
}

func (n *gtsNode) Utf8Text(source []byte) string { return n.n.Text(source) }

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
