package codesignal

import (
	"path"
	"sort"
)

// undeterminedContinuityEvidenceNote is the rename/copy counterpart of
// indeterminateLifecycleEvidenceNote: no project_lifecycle_indeterminate
// diagnostic exists for this cause, so the note points at the per-change one.
const undeterminedContinuityEvidenceNote = ` (lifecycle is "unknown": see the ` + DiagKindProjectChangeLifecycleIndeterminate + ` diagnostic for why)`

// undeterminedContinuity indexes Input.UndeterminedContinuity by comparison
// side: head holds each rename/copy's new path, base its previous path.
type undeterminedContinuity struct {
	head map[string]struct{}
	base map[string]struct{}
}

func newUndeterminedContinuity(pairs []PathContinuity) undeterminedContinuity {
	c := undeterminedContinuity{head: map[string]struct{}{}, base: map[string]struct{}{}}
	for _, pair := range pairs {
		if pair.Path != "" {
			c.head[pair.Path] = struct{}{}
		}
		if pair.PreviousPath != "" {
			c.base[pair.PreviousPath] = struct{}{}
		}
	}
	return c
}

// headPath returns the first rename/copy destination change touches. A
// change touches a path through any location, and through the importer or
// importee its identity embeds: a moved importee changes the key while
// every location still points at the unmoved importing file.
func (c undeterminedContinuity) headPath(change ProjectChange) (string, bool) {
	return firstMovedPathTouchedBy(change, c.head)
}

// basePath returns the first rename/copy source change touches.
func (c undeterminedContinuity) basePath(change ProjectChange) (string, bool) {
	return firstMovedPathTouchedBy(change, c.base)
}

func firstMovedPathTouchedBy(change ProjectChange, moved map[string]struct{}) (string, bool) {
	if len(moved) == 0 {
		return "", false
	}
	for _, p := range projectChangeLocationPaths(change) {
		if _, ok := moved[p]; ok {
			return p, true
		}
	}
	identities := projectChangeIdentityPaths(change)
	if len(identities) == 0 {
		return "", false
	}
	for _, p := range sortedPaths(moved) {
		for _, identity := range identities {
			// TypeScript identities are files; Go identities are package
			// directories, so a moved file also touches its directory.
			if identity == p || identity == path.Dir(p) {
				return p, true
			}
		}
	}
	return "", false
}

func projectChangeLocationPaths(change ProjectChange) []string {
	paths := []string{change.PrimaryAnchor.Path}
	for _, loc := range change.RelatedLocations {
		paths = append(paths, loc.Path)
	}
	for _, step := range change.PathSteps {
		for _, loc := range step.SourceLocations {
			paths = append(paths, loc.Path)
		}
	}
	return paths
}

func projectChangeIdentityPaths(change ProjectChange) []string {
	var paths []string
	for _, key := range []string{"importer", "importee"} {
		if p := change.MachineEvidence[key]; p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func sortedPaths(set map[string]struct{}) []string {
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// continuityDegradedDiagnostic names the rename/copy path that kept a project
// change from being classified introduced (side "head") or resolved (side
// "base").
func continuityDegradedDiagnostic(path, side, revision string) Diagnostic {
	return Diagnostic{
		Path:     path,
		Kind:     DiagKindProjectChangeLifecycleIndeterminate,
		Message:  "project change lifecycle is \"unknown\": rename/copy continuity of this path was not determined at " + sideRevisionLabel(side, revision),
		Side:     side,
		Revision: revision,
	}
}

// sideRevisionLabel renders "<side> revision <revision>", dropping the
// revision when a library caller left Scope.Revision or Scope.Base empty.
func sideRevisionLabel(side, revision string) string {
	if revision == "" {
		return side + " revision"
	}
	return side + " revision " + revision
}

func appendDistinctDiagnostics(diagnostics []Diagnostic, seen map[Diagnostic]struct{}, candidates ...Diagnostic) []Diagnostic {
	for _, d := range candidates {
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		diagnostics = append(diagnostics, d)
	}
	return diagnostics
}
