package evaldetect

// Bypass is one layer-bypass path.
type Bypass struct {
	Finding
	Source             string
	Sink               string
	Witness            []string
	ViaForbiddenImport bool
}

// BypassReport is the bypass detector's result.
type BypassReport struct {
	Bypasses   []Bypass
	Unresolved int
	Reason     string
}

const layerBypass = "architecture.layer_bypass"

// maxCallGraphNodes bounds the search: a program with more local functions
// is refused rather than searched in part.
var maxCallGraphNodes = 100000

// GoLayerBypass reports every static call path from an upstream source to a
// downstream sink that touches no function in the policy's required layer.
// The grader defines sources and sinks from the policy alone (bypassLayers),
// independently of Coach, whose own sinks may be narrower. Only forbidden
// pairs orient the layers, so a policy without a layer on each side of the
// required layer is reported with a reason rather than searched.
func GoLayerBypass(root string, policy Policy) (BypassReport, error) {
	if policy.requiredLayer == "" {
		return BypassReport{Reason: "the policy names no required_layer"}, nil
	}
	if upstream, downstream := policy.bypassLayers(); len(upstream) == 0 || len(downstream) == 0 {
		return BypassReport{Reason: "the policy's forbidden_imports give no layer above or below the required layer"}, nil
	}
	program, err := loadGoProgram(root)
	if err != nil {
		return BypassReport{}, err
	}
	graph, err := buildCallGraph(program)
	if err != nil {
		return BypassReport{}, err
	}
	return BypassReport{Bypasses: searchBypasses(graph, policy), Unresolved: graph.unresolved + unfollowedBodylessCalls(graph, policy)}, nil
}

// BypassDisagreement is a bypass only one side reports.
type BypassDisagreement struct {
	Path   string
	Side   string
	Source string
	Sink   string
}

// BypassComparison is the detector's bypasses compared with Coach's.
type BypassComparison struct {
	Agreed        []string
	Disagreements []BypassDisagreement
	Graded        bool
}

// CompareBypasses matches the detector's bypasses with Coach's by the file
// they start in. A bypass only one side reports is recorded as a
// disagreement and never graded as a defect, because the grader's sinks are
// broader than Coach's may be.
func CompareBypasses(detector []Bypass, coach []Finding) BypassComparison {
	coachPaths := map[string]bool{}
	for _, finding := range coach {
		coachPaths[finding.Path] = true
	}
	detectorPaths := map[string]bool{}
	var comparison BypassComparison
	for _, bypass := range detector {
		detectorPaths[bypass.Path] = true
		if coachPaths[bypass.Path] {
			comparison.Agreed = append(comparison.Agreed, bypass.Path)
			continue
		}
		comparison.Disagreements = append(comparison.Disagreements, BypassDisagreement{Path: bypass.Path, Side: "detector_only", Source: bypass.Source, Sink: bypass.Sink})
	}
	for _, finding := range coach {
		if !detectorPaths[finding.Path] {
			comparison.Disagreements = append(comparison.Disagreements, BypassDisagreement{Path: finding.Path, Side: "coach_only"})
		}
	}
	return comparison
}
