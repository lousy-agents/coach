package codesignal

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

// unevaluatedTSEdgeDiagnostics reports the edges layer rules could not place
// on a layer. Only edges whose importer sits inside a declared layer are
// considered: an importer outside every layer has no rule that its imports
// could break, so reporting them would only add noise.
func unevaluatedTSEdgeDiagnostics(edges []domain.ImportEdge, policy LayerPolicy) []Diagnostic {
	var diagnostics []Diagnostic
	externalEdges, nonSourceEdges := 0, 0
	for _, edge := range edges {
		if !evaluatedTSEdgeKind(edge.Kind, policy.ExcludeTypeOnlyEdges) {
			continue
		}
		importer, ok := strings.CutPrefix(edge.From, "file:")
		if !ok {
			continue
		}
		if _, inLayer := matchLayer(policy.Layers, importer); !inLayer {
			continue
		}
		if specifier, unresolved := strings.CutPrefix(edge.To, "unresolved:"); unresolved {
			if isNonSourceSpecifier(specifier) {
				nonSourceEdges++
			} else {
				diagnostics = append(diagnostics, unresolvedImportDiagnostic(importer, specifier, edge.Site))
			}
		} else if strings.HasPrefix(edge.To, "external:") {
			externalEdges++
		}
	}

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		if diagnostics[i].Location.StartRow != diagnostics[j].Location.StartRow {
			return diagnostics[i].Location.StartRow < diagnostics[j].Location.StartRow
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	if externalEdges+nonSourceEdges > 0 {
		diagnostics = append(diagnostics, notEvaluatedImportsDiagnostic(externalEdges, nonSourceEdges))
	}
	return diagnostics
}

// nonSourceExtensions lists files that are clearly not code. The sidecar only
// receives TypeScript sources, so an import of one of these never resolves
// there even though the file exists, and reporting it as an unresolved import
// would mark nearly every web project's verdict incomplete. Such a file cannot
// hide a forbidden code edge. A .js, .cjs or .mjs target stays diagnosed: it
// could be a broken path to code.
var nonSourceExtensions = map[string]struct{}{
	".json": {}, ".css": {}, ".scss": {}, ".sass": {}, ".less": {}, ".styl": {},
	".svg": {}, ".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".avif": {}, ".ico": {}, ".bmp": {},
	".woff": {}, ".woff2": {}, ".ttf": {}, ".otf": {}, ".eot": {},
	".mp3": {}, ".mp4": {}, ".webm": {}, ".ogg": {}, ".wav": {},
	".html": {}, ".md": {}, ".mdx": {}, ".txt": {}, ".csv": {}, ".yaml": {}, ".yml": {}, ".toml": {},
	".graphql": {}, ".gql": {}, ".wasm": {},
}

func isNonSourceSpecifier(specifier string) bool {
	if query := strings.IndexAny(specifier, "?#"); query >= 0 {
		specifier = specifier[:query]
	}
	_, ok := nonSourceExtensions[strings.ToLower(path.Ext(specifier))]
	return ok
}

func unresolvedImportDiagnostic(importer, specifier, site string) Diagnostic {
	location := parseSiteLocation(site).Location
	return Diagnostic{
		Path:     importer,
		Kind:     DiagKindProjectImportUnresolved,
		Message:  fmt.Sprintf("import of %q from %s did not resolve within the snapshot, so layer rules cannot tell whether it crosses a layer; the layer verdict is incomplete", specifier, importer),
		Location: &location,
	}
}

func notEvaluatedImportsDiagnostic(external, nonSource int) Diagnostic {
	var message string
	switch {
	case nonSource == 0 && external == 1:
		message = "1 import edge to an external package was not evaluated by layer rules"
	case nonSource == 0:
		message = fmt.Sprintf("%d import edges to external packages were not evaluated by layer rules", external)
	case external == 0 && nonSource == 1:
		message = "1 import edge to a non-source file was not evaluated by layer rules"
	case external == 0:
		message = fmt.Sprintf("%d import edges to non-source files were not evaluated by layer rules", nonSource)
	default:
		message = fmt.Sprintf("%d import edges to external packages or non-source files were not evaluated by layer rules", external+nonSource)
	}
	return Diagnostic{Kind: DiagKindProjectExternalImportsNotEvaluated, Message: message}
}

// isProjectEdgeDiagnosticKind recognizes the two kinds above on either side of
// a diff. Their Path names a file that is analyzed, so they must never count
// that file as unanalyzed.
func isProjectEdgeDiagnosticKind(kind string) bool {
	switch strings.TrimPrefix(kind, "base_") {
	case DiagKindProjectImportUnresolved, DiagKindProjectExternalImportsNotEvaluated:
		return true
	}
	return false
}
