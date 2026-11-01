package codesignal

import (
	"sort"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// ruleLayerViolationID identifies the architecture.layer_violation
// rule in ProjectChange.RuleID.
const ruleLayerViolationID = "architecture.layer_violation"

const layerViolationWhyItMatters = "Import edges that cross a configured layer boundary erode the architecture the team claims to follow, so drift compounds across packages that look fine in isolation."

const layerViolationRecommendation = "Move the dependency behind an allowed boundary (interface in the lower layer, invert the import, or relocate the shared type), or update the explicit layer policy if the edge is intentional."

// ArchitectureLayer names a policy layer and its repository-relative
// directory prefixes. Prefixes are assumed already validated as
// non-empty, unique, and mutually non-overlapping by the caller.
type ArchitectureLayer struct {
	Name     string
	Prefixes []string
}

// ForbiddenLayerImport is one directed "layer From must not import layer To" rule.
type ForbiddenLayerImport struct {
	From string
	To   string
}

// LayerPolicy is a decoded, already-schema-validated layer configuration.
type LayerPolicy struct {
	Layers           []ArchitectureLayer
	ForbiddenImports []ForbiddenLayerImport

	// ExcludeTypeOnlyEdges is named for what it removes, so the zero value
	// keeps the default of evaluating TypeScript type-only edges. The Go
	// evaluator ignores it because Go has no type-only import edge.
	ExcludeTypeOnlyEdges bool
}

// EvaluateGoLayerViolations evaluates model's internal Go import edges
// against policy and returns one architecture.layer_violation ProjectChange
// per violating (importer package directory, importee package directory)
// pair -- multiple edges/sites between the same pair collapse into a single
// ProjectChange rather than one per edge.
//
// Only ImportEdges with Kind == "internal" are ever eligible; every other
// Kind (stdlib, external, replaced, unresolved, excluded) is silently
// skipped. An edge whose importer or importee package directory does not
// map to any configured layer is likewise silently skipped -- that is an
// expected, non-error outcome (partial layer coverage), not a diagnostic.
// Consequently the returned diagnostics are always nil in the current
// design.
//
// If policy has no Layers or no ForbiddenImports, EvaluateGoLayerViolations
// returns (nil, nil) immediately: there is no policy to evaluate.
func EvaluateGoLayerViolations(model domain.Model, policy LayerPolicy, ruleVersion, backendVersion, configDigest string) ([]ProjectChange, []Diagnostic) {
	if len(policy.Layers) == 0 || len(policy.ForbiddenImports) == 0 {
		return nil, nil
	}

	groups, groupLayers := groupForbiddenInternalEdges(model.ImportEdges, policy)
	if len(groups) == 0 {
		return nil, nil
	}

	keys := sortedLayerPairKeys(groups)
	changes := make([]ProjectChange, 0, len(keys))
	for _, key := range keys {
		layers := groupLayers[key]
		changes = append(changes, layerViolationChange(key, groups[key], layers[0], layers[1], ruleVersion, backendVersion, configDigest, ""))
	}
	return changes, nil
}

// EvaluateTypeScriptLayerViolations is the TypeScript/TSX analog of
// EvaluateGoLayerViolations: it evaluates model's file-addressed TS/TSX
// import edges (value-level import and reexport, type-only, and string-literal
// dynamic import() and require() calls) against policy, producing
// architecture.layer_violation ProjectChanges with the same field shape as the
// Go evaluator plus a MachineEvidence["language"] = "typescript" entry.
//
// An edge is eligible only when its Kind is one evaluatedTSEdgeKind accepts
// and both From and To carry the "file:" prefix (see forbiddenTSPair).
// "type_only" edges are evaluated because a layer rule forbids a
// compile-time dependency just as it forbids a runtime one (coach#562).
// Dynamic import() and require() are evaluated like static imports
// (coach#561). The project sidecar only ever emits a "file:"-addressed To
// together with Resolution "snapshot"
// (js/semantics/src/project-sidecar/edges-resolve.ts), so the prefix check
// alone is sufficient to select snapshot-resolved edges.
//
// An edge from a file inside a declared layer to an "unresolved:" target gets
// one DiagKindProjectImportUnresolved diagnostic, and the edges to
// "external:" targets are counted in a single
// DiagKindProjectExternalImportsNotEvaluated diagnostic. Neither is a
// violation. Callers stamp Side and Revision.
//
// If policy has no Layers or no ForbiddenImports, EvaluateTypeScriptLayerViolations
// returns (nil, nil) immediately: there is no policy to evaluate.
func EvaluateTypeScriptLayerViolations(model domain.Model, policy LayerPolicy, ruleVersion, backendVersion, configDigest string) ([]ProjectChange, []Diagnostic) {
	if len(policy.Layers) == 0 || len(policy.ForbiddenImports) == 0 {
		return nil, nil
	}

	diagnostics := unevaluatedTSEdgeDiagnostics(model.ImportEdges, policy)
	groups, groupLayers := groupForbiddenTSEdges(model.ImportEdges, policy)
	if len(groups) == 0 {
		return nil, diagnostics
	}

	keys := sortedLayerPairKeys(groups)
	changes := make([]ProjectChange, 0, len(keys))
	for _, key := range keys {
		layers := groupLayers[key]
		changes = append(changes, layerViolationChange(key, groups[key], layers[0], layers[1], ruleVersion, backendVersion, configDigest, "typescript"))
	}
	return changes, diagnostics
}

// allEdgesTypeOnly marks findings the ExcludeTypeOnlyEdges option would
// remove entirely. A pair that also has a value-level edge survives the
// option, so it must not carry the marker.
func allEdgesTypeOnly(edges []domain.ImportEdge) bool {
	for _, edge := range edges {
		if edge.Kind != "type_only" {
			return false
		}
	}
	return len(edges) > 0
}

// layerViolationChange builds the shared architecture.layer_violation
// ProjectChange shape for both language evaluators. language is added to
// MachineEvidence["language"] only when non-empty, so Go's evaluator (which
// passes "") keeps its already-frozen MachineEvidence keys (issue #211)
// while EvaluateTypeScriptLayerViolations gains a "language": "typescript"
// entry.
func layerViolationChange(key layerPairKey, edges []domain.ImportEdge, layerFromName, layerToName, ruleVersion, backendVersion, configDigest, language string) ProjectChange {
	sites := make([]string, 0, len(edges))
	for _, edge := range edges {
		sites = append(sites, edge.Site)
	}
	sort.Strings(sites)

	primary := parseSiteLocation(sites[0])
	var related []ProjectLocation
	for _, site := range sites[1:] {
		related = append(related, parseSiteLocation(site))
	}

	machineEvidence := map[string]string{
		"importer":   key.importer,
		"importee":   key.importee,
		"layer_from": layerFromName,
		"layer_to":   layerToName,
		"rule":       layerFromName + "->" + layerToName,
	}
	if language != "" {
		machineEvidence["language"] = language
	}
	if allEdgesTypeOnly(edges) {
		machineEvidence["removed_if_set"] = "exclude_type_only_edges"
	}

	return ProjectChange{
		SemanticKey:      "architecture.layer_violation:" + key.importer + "->" + key.importee,
		RuleID:           ruleLayerViolationID,
		RuleVersion:      ruleVersion,
		BackendVersion:   backendVersion,
		ConfigDigest:     configDigest,
		Kind:             "architecture.layer_violation",
		Category:         Category("architecture"),
		Severity:         Severity("advisory"),
		Confidence:       Confidence("high"),
		MachineEvidence:  machineEvidence,
		PrimaryAnchor:    primary,
		RelatedLocations: related,
		Evidence:         key.importer + " imports " + key.importee + " (layer " + layerFromName + " -> " + layerToName + " is forbidden)",
		WhyItMatters:     layerViolationWhyItMatters,
		Recommendation:   layerViolationRecommendation,
		Provenance:       Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_violation"},
	}
}

// parseSiteLocation splits an ImportEdge.Site ("<file>:<1-based line>") on
// the last colon (file paths never contain one, but this stays defensive)
// and converts the 1-based line to the 0-based StartRow ProjectLocation
// uses elsewhere in this package.
func parseSiteLocation(site string) ProjectLocation {
	idx := strings.LastIndex(site, ":")
	if idx < 0 {
		return ProjectLocation{Path: site}
	}
	path := site[:idx]
	row, err := strconv.Atoi(site[idx+1:])
	if err != nil || row <= 0 {
		return ProjectLocation{Path: path}
	}
	return ProjectLocation{Path: path, Location: semantics.Location{StartRow: uint(row - 1)}}
}
