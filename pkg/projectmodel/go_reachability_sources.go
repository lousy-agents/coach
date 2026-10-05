package projectmodel

import (
	"context"
	"go/types"
)

// findGoReachabilitySourcesFromLoaded walks loaded's local functions and
// returns every function whose signature is identical to
// net/http.HandlerFunc's underlying func(http.ResponseWriter, *http.Request).
// Source identification needs each function's own signature, which
// CallGraphResult's From/To strings do not carry, so this walk stays
// separate from the call-graph walk rather than growing CallFact.
func findGoReachabilitySourcesFromLoaded(ctx context.Context, loaded *loadedGoSnapshot) ([]string, bool, []Diagnostic) {
	if ctx.Err() != nil {
		return nil, false, []Diagnostic{{Code: DiagReachabilityBudgetExceeded}}
	}

	search := goReachabilitySourceSearch{
		seen:     map[string]bool{},
		complete: loaded.discovery.Complete,
	}
	for _, root := range loaded.roots {
		if search.collectRoot(ctx, loaded, root) {
			break
		}
	}
	return mapKeysSorted(search.seen), search.complete, search.diagnostics
}

type goReachabilitySourceSearch struct {
	seen        map[string]bool
	diagnostics []Diagnostic
	complete    bool
}

func (s *goReachabilitySourceSearch) collectRoot(ctx context.Context, loaded *loadedGoSnapshot, root loadedGoRoot) (stop bool) {
	if ctx.Err() != nil {
		s.complete = false
		s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
		return true
	}
	if root.loadErr != nil {
		s.complete = false
		s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilitySourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
		return false
	}
	for _, p := range root.pkgs {
		if len(p.Errors) > 0 {
			s.complete = false
		}
	}
	handlerSig := httpHandlerFuncSignature(root.prog, root.pkgs)
	if handlerSig == nil {
		return false
	}
	for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
		if ctx.Err() != nil {
			s.complete = false
			s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
			return true
		}
		if types.Identical(fn.Signature, handlerSig) {
			s.seen[fn.RelString(nil)] = true
		}
	}
	return false
}
