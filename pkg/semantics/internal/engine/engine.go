// Package engine defines the narrow tree-sitter surface pkg/semantics
// relies on, so the package can be built against more than one underlying
// parser implementation. It is deliberately not a general-purpose
// tree-sitter binding: it exposes exactly the Node/Tree/Parser/Query
// operations query.go, ts_query.go, features.go, ts_features.go, and
// parser.go use today -- no NamedChild, no TreeCursor/Walk, no query
// predicates, no incremental parsing.
//
// This package lives under pkg/semantics/internal so it is importable only
// from within pkg/semantics itself; it is not part of any public API.
package engine

// Node is one node in a parsed syntax tree.
type Node interface {
	Kind() string
	HasError() bool
	IsError() bool
	IsMissing() bool
	ChildCount() int
	// Child returns the i'th child, or nil if i is out of range.
	Child(i int) Node
	// ChildByFieldName returns the child bound to name, or nil if absent.
	ChildByFieldName(name string) Node
	// Parent returns the parent node, or nil for the root.
	Parent() Node
	StartByte() uint
	EndByte() uint
	StartPoint() (row, col uint)
	EndPoint() (row, col uint)
	Utf8Text(source []byte) string
}

// Tree is a parsed syntax tree. The caller owns the returned Tree and must
// call Close when done with it.
type Tree interface {
	RootNode() Node
	Close()
}

// Parser parses source bytes into a Tree for the Language it was created
// for.
type Parser interface {
	Parse(content []byte) (Tree, error)
}

// QueryCapture is one capture within a QueryMatch.
type QueryCapture struct {
	Node  Node
	Index uint32
}

// QueryMatch is one match yielded during query iteration.
type QueryMatch struct {
	Captures []QueryCapture
}

// QueryMatches iterates a Query's matches against one Node/source pair.
// Next returns nil once iteration is exhausted.
type QueryMatches interface {
	Next() *QueryMatch
}

// Query is a compiled tree-sitter query. A Query is valid only against
// trees parsed with the exact Language it was compiled against; this
// invariant is preserved by construction, since every Query is created via
// that Language's own NewQuery method.
type Query interface {
	Close()
}

// QueryCursor runs a Query against one Node/source pair. The caller owns
// the returned QueryCursor and must call Close when done with it.
type QueryCursor interface {
	Matches(query Query, root Node, source []byte) QueryMatches
	Close()
}

// Language is a backend-bound handle for one of pkg/semantics's supported
// grammars. It is the sole seam between pkg/semantics's language.go and a
// concrete backend implementation.
type Language interface {
	NewParser() (Parser, error)
	NewQuery(source string) (Query, error)
	NewQueryCursor() QueryCursor
}
