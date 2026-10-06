package semantics

import (
	"regexp"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// constructorFuncNameRe matches function names that look like Go
// constructors: "New" followed by an uppercase letter, digit, or
// underscore (e.g. NewFoo, New2, New_thing), or "New" alone. It
// deliberately does not match names like "Newton", where the character
// after "New" is a lowercase letter.
var constructorFuncNameRe = regexp.MustCompile(`^New([A-Z0-9_]|$)`)

// checkConstructorFunc emits a "constructor_func" Finding (AC-3.5) if decl's
// name field matches constructorFuncNameRe.
func (c *featureCollector) checkConstructorFunc(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(source)
	if !constructorFuncNameRe.MatchString(name) {
		return
	}
	c.findings = append(c.findings, Finding{
		Kind:     "constructor_func",
		Name:     name,
		Location: locationFromNode(decl),
	})
}
