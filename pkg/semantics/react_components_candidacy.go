package semantics

import (
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactBuildComponentFacts applies the remaining candidacy gates (name
// present and PascalCase, JSX body, client gate) to cand and, if it passes,
// extracts its useState bindings and coordination facts.
func reactBuildComponentFacts(cand reactCandidate, hasDirective bool, source []byte) (ReactComponentFacts, bool) {
	if cand.funcNode == nil || !isPascalCaseName(cand.name) {
		return ReactComponentFacts{}, false
	}
	body := cand.funcNode.ChildByFieldName("body")
	if body == nil {
		return ReactComponentFacts{}, false
	}
	if !reactScopeContainsJSX(body, source) {
		return ReactComponentFacts{}, false
	}

	clientKind := ""
	switch {
	case hasDirective:
		clientKind = "use_client_directive"
	case reactScopeInvokesHook(body, source):
		clientKind = "hooks_and_jsx"
	default:
		return ReactComponentFacts{}, false
	}

	useState := reactExtractUseState(body, source)

	return ReactComponentFacts{
		Name:                   cand.name,
		Location:               locationFromNode(cand.funcNode),
		ClientKind:             clientKind,
		UseState:               useState,
		CoordinatedTransitions: reactExtractCoordinatedTransitions(body, source, useState),
		WorkspaceBranches:      reactExtractWorkspaceBranches(body, source),
		ImperativeUI:           reactExtractImperativeUI(body, source),
		SharedPanelDeps:        reactExtractSharedPanelDeps(body, source, useState),
	}, true
}

func isPascalCaseName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// reactHookNamePattern matches a hook-shaped identifier/property name: an
// uppercase letter immediately after the "use" prefix (e.g. useState,
// useEffect), per the React hooks naming convention this package relies on
// for the no-directive client gate.
var reactHookNamePattern = regexp.MustCompile(`^use[A-Z]`)

func reactIsHookCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		return reactHookNamePattern.MatchString(fn.Utf8Text(source))
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		return reactHookNamePattern.MatchString(prop.Utf8Text(source))
	default:
		return false
	}
}
