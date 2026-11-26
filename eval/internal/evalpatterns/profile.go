package evalpatterns

// ExtractorVersion is the version of the measures a profile records; a
// change to any measure changes it, and so the profile's identity.
const ExtractorVersion = "0.1.0"

// distribution counts observations per integer bucket, keyed by the bucket
// written in decimal.
type distribution map[string]int

func (d distribution) add(bucket int) {
	d[itoa(bucket)]++
}

type entryKinds struct {
	HTTPHandler int `json:"http_handler"`
	Method      int `json:"method"`
	Function    int `json:"function"`
}

type constructorShapes struct {
	AssignsOnly int `json:"assigns_only"`
	DoesWork    int `json:"does_work"`
}

type roleStats struct {
	Packages distribution `json:"packages"`
}

type dimensions struct {
	Layers distribution `json:"layers"`
	Entry  entryKinds   `json:"entry"`
	Depth  distribution `json:"depth"`
	// Trap, Field and Direction are measures syntax cannot take, so they
	// stay null in every profile (AC-2.7).
	Trap      *string `json:"trap"`
	Field     *string `json:"field"`
	Direction *string `json:"direction"`
}

type shapes struct {
	Nesting       distribution      `json:"nesting"`
	Cognitive     distribution      `json:"cognitive"`
	BranchDensity distribution      `json:"branch_density"`
	Constructor   constructorShapes `json:"constructor"`
}

type notCovered struct {
	Shape  string `json:"shape"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// The closed list of reasons a shape is not covered.
const (
	outOfRange                 = "out_of_range"
	noTotalOrder               = "no_total_order"
	policyDeclaresNoLayers     = "policy_declares_no_layers"
	policyLayersMatchNoPackage = "policy_layers_match_no_package"
)

// profile is the derived statistics of one reference at one commit. Its
// only free text is the label and the licence.
type profile struct {
	SchemaVersion    string               `json:"schema_version"`
	Identity         string               `json:"identity,omitempty"`
	Label            string               `json:"label"`
	SHA              string               `json:"sha"`
	Licence          string               `json:"licence"`
	ExtractorVersion string               `json:"extractor_version"`
	Roles            map[string]roleStats `json:"roles"`
	Dimensions       dimensions           `json:"dimensions"`
	Shapes           shapes               `json:"shapes"`
	NotCovered       []notCovered         `json:"not_covered"`
}

func newProfile(reference Reference) profile {
	return profile{
		SchemaVersion: "1", Label: reference.Label, SHA: reference.SHA, Licence: reference.Licence,
		ExtractorVersion: ExtractorVersion, Roles: map[string]roleStats{},
		Dimensions: dimensions{Layers: distribution{}, Depth: distribution{}},
		Shapes:     shapes{Nesting: distribution{}, Cognitive: distribution{}, BranchDensity: distribution{}},
		NotCovered: []notCovered{},
	}
}
