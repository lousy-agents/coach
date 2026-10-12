// Adapts github.com/odvcencio/gotreesitter (a from-scratch, CGO-free
// reimplementation of the Tree-sitter runtime) to the engine interfaces.
package engine

import (
	"fmt"
	"sync"

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
	return &gtsLanguage{entry: entry, wantsForest: name == "typescript" || name == "tsx"}
}

type gtsLanguage struct {
	entry *grammars.LangEntry
	// wantsForest is applied lazily, in lang(), not eagerly in
	// GoTreeSitterLanguage: entry.Language is a lazy loader (see
	// grammars.LangEntry), and languageRegistry constructs every registered
	// Language at package-init time, so touching it eagerly would force
	// every language's grammar blob to decompress on import even for
	// callers that only ever parse one of them.
	wantsForest bool
	// forestOnce guards the WantsForest write below: entry.Language()
	// returns a cached, shared *gotreesitter.Language (not a fresh value per
	// call), and lang() runs concurrently across goroutines (AnalyzeBytes
	// creates its own Parser per call, sharing this Language, and is
	// documented safe for concurrent use), so an unguarded write here would
	// race with concurrent reads/writes of the same field.
	forestOnce sync.Once
}

func (l *gtsLanguage) lang() *gotreesitter.Language {
	lang := l.entry.Language()
	if l.wantsForest {
		l.forestOnce.Do(func() {
			// gotreesitter's plain parse path misparses plain-identifier
			// default parameters (e.g. `function f(x = 1) {}`) and
			// array-destructuring defaults (e.g. `const [a = 2] = z;`) as
			// syntax errors. WantsForest is gotreesitter's own documented
			// opt-in (see gotreesitter's language.go) that routes parsing
			// through its GSS-forest GLR path, which handles these shapes
			// correctly and falls back to the existing parser automatically
			// on any forest failure or error node, so it's a strict
			// improvement with no regression risk.
			lang.WantsForest = true
		})
	}
	return lang
}

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
