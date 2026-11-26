package evaldetect

import "errors"

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

var maxCallGraphNodes = 100000

func GoLayerBypass(root string, policy Policy) (BypassReport, error) {
	return BypassReport{}, errors.New("not implemented")
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

func CompareBypasses(detector []Bypass, coach []Finding) BypassComparison {
	return BypassComparison{Graded: true}
}
