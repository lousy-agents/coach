package engine

import (
	"github.com/odvcencio/gotreesitter"
)

// gtsNode carries lang alongside its *gotreesitter.Node because, unlike
// go-tree-sitter's Node, gotreesitter's Type/ChildByFieldName resolve node
// kind names and field lookups through an explicit *Language argument
// rather than a language the node is intrinsically bound to.
type gtsNode struct {
	n    *gotreesitter.Node
	lang *gotreesitter.Language
}

func (n *gtsNode) Kind() string { return n.n.Type(n.lang) }

func (n *gtsNode) HasError() bool { return n.n.HasError() }

func (n *gtsNode) IsError() bool { return n.n.IsError() }

func (n *gtsNode) IsMissing() bool { return n.n.IsMissing() }

func (n *gtsNode) ChildCount() int { return n.n.ChildCount() }

func (n *gtsNode) StartByte() uint { return uint(n.n.StartByte()) }

func (n *gtsNode) EndByte() uint { return uint(n.n.EndByte()) }

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
