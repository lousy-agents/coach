//go:build cgo

// The CGO backend: adapts github.com/tree-sitter/go-tree-sitter (which
// binds Tree-sitter's C runtime) to the engine interfaces. This is the
// default backend on any platform where CGO is available; see
// language_gotreesitter.go for when it is not compiled in.
package engine

import (
	"fmt"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// CGOLanguage adapts a go-tree-sitter grammar accessor -- the
// func() unsafe.Pointer signature every tree-sitter-<language> Go binding
// exposes -- into a Language. This unsafe.Pointer (a raw *C.TSLanguage
// handle) is the only CGO-specific value that crosses out of this file.
func CGOLanguage(grammar func() unsafe.Pointer) Language {
	return cgoLanguage{grammar: grammar}
}

type cgoLanguage struct {
	grammar func() unsafe.Pointer
}

func (l cgoLanguage) tsLanguage() *tree_sitter.Language {
	return tree_sitter.NewLanguage(l.grammar())
}

func (l cgoLanguage) NewParser() (Parser, error) {
	p := tree_sitter.NewParser()
	if p == nil {
		return nil, fmt.Errorf("engine: tree_sitter.NewParser returned nil")
	}
	if err := p.SetLanguage(l.tsLanguage()); err != nil {
		return nil, fmt.Errorf("engine: setting grammar language: %w", err)
	}
	return &cgoParser{p: p}, nil
}

func (l cgoLanguage) NewQuery(source string) (Query, error) {
	q, err := tree_sitter.NewQuery(l.tsLanguage(), source)
	if err != nil {
		return nil, err
	}
	return &cgoQuery{q: q}, nil
}

func (l cgoLanguage) NewQueryCursor() QueryCursor {
	return &cgoQueryCursor{c: tree_sitter.NewQueryCursor()}
}

// cgoParser wraps a single-use *tree_sitter.Parser. It is closed inside
// Parse itself, immediately after parsing, mirroring the original
// parser.go pipeline's per-call parser lifecycle: the Parser is discarded
// once it has produced a Tree, which the caller owns independently.
type cgoParser struct {
	p *tree_sitter.Parser
}

func (p *cgoParser) Parse(content []byte) (Tree, error) {
	tree := p.p.Parse(content, nil)
	p.p.Close()
	if tree == nil {
		return nil, nil
	}
	return &cgoTree{t: tree}, nil
}

type cgoTree struct {
	t *tree_sitter.Tree
}

func (t *cgoTree) RootNode() Node { return &cgoNode{n: t.t.RootNode()} }
func (t *cgoTree) Close()         { t.t.Close() }

type cgoNode struct {
	n *tree_sitter.Node
}

func (n *cgoNode) Kind() string    { return n.n.Kind() }
func (n *cgoNode) HasError() bool  { return n.n.HasError() }
func (n *cgoNode) IsError() bool   { return n.n.IsError() }
func (n *cgoNode) IsMissing() bool { return n.n.IsMissing() }
func (n *cgoNode) ChildCount() int { return int(n.n.ChildCount()) }
func (n *cgoNode) StartByte() uint { return n.n.StartByte() }
func (n *cgoNode) EndByte() uint   { return n.n.EndByte() }

func (n *cgoNode) Child(i int) Node {
	c := n.n.Child(uint(i))
	if c == nil {
		return nil
	}
	return &cgoNode{n: c}
}

func (n *cgoNode) ChildByFieldName(name string) Node {
	c := n.n.ChildByFieldName(name)
	if c == nil {
		return nil
	}
	return &cgoNode{n: c}
}

func (n *cgoNode) Parent() Node {
	p := n.n.Parent()
	if p == nil {
		return nil
	}
	return &cgoNode{n: p}
}

func (n *cgoNode) StartPoint() (row, col uint) {
	p := n.n.StartPosition()
	return p.Row, p.Column
}

func (n *cgoNode) EndPoint() (row, col uint) {
	p := n.n.EndPosition()
	return p.Row, p.Column
}

func (n *cgoNode) Utf8Text(source []byte) string { return n.n.Utf8Text(source) }

type cgoQuery struct {
	q *tree_sitter.Query
}

func (q *cgoQuery) Close() { q.q.Close() }

type cgoQueryCursor struct {
	c *tree_sitter.QueryCursor
}

func (c *cgoQueryCursor) Close() { c.c.Close() }

func (c *cgoQueryCursor) Matches(query Query, root Node, source []byte) QueryMatches {
	q := query.(*cgoQuery).q
	r := root.(*cgoNode).n
	return &cgoQueryMatches{m: c.c.Matches(q, r, source)}
}

type cgoQueryMatches struct {
	m tree_sitter.QueryMatches
}

func (m *cgoQueryMatches) Next() *QueryMatch {
	match := m.m.Next()
	if match == nil {
		return nil
	}
	captures := make([]QueryCapture, len(match.Captures))
	for i := range match.Captures {
		// Index into the original slice element, not a loop-copy variable,
		// so the address taken is the actual captured Node's storage.
		captures[i] = QueryCapture{
			Node:  &cgoNode{n: &match.Captures[i].Node},
			Index: match.Captures[i].Index,
		}
	}
	return &QueryMatch{Captures: captures}
}
