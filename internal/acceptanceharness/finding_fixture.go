package acceptanceharness

// FindingSource distinguishes a finding a deterministic analysis pass
// produced from one an agent proposed or contextualized. Per
// docs/architecture/system-overview.md's ADR table ("Separate
// deterministic/agent provenance" for reproducibility and clarity) and C4
// 3C ("An agent may contextualize or propose a governed suppression, but
// cannot overwrite a deterministic result; its separate record uses
// source=agent"), the two provenances must remain distinguishable in a
// fixture rather than blended into one finding shape.
type FindingSource string

const (
	// FindingSourceDeterministic marks a finding produced by deterministic
	// structural analysis.
	FindingSourceDeterministic FindingSource = "deterministic"
	// FindingSourceAgent marks a finding an agent proposed or
	// contextualized, kept as a separate record rather than overwriting a
	// deterministic result.
	FindingSourceAgent FindingSource = "agent"
)

// FindingFixture is the minimal shape a golden report fixture needs to
// represent one finding: enough to prove provenance separation and rule
// identity, without carrying full production Report/Signal fields (this is
// a fixture/test vocabulary, not a production schema -- pkg/codesignal
// owns that separately).
type FindingFixture struct {
	SchemaVersion int           `json:"schema_version"`
	Source        FindingSource `json:"source"`
	RuleID        string        `json:"rule_id"`
	Path          string        `json:"path"`
	Severity      string        `json:"severity"`
}

// NewFindingFixture builds a FindingFixture stamped with the current
// ReportFixtureSchemaVersion, so callers can't accidentally omit or
// mismatch it.
func NewFindingFixture(source FindingSource, ruleID, path, severity string) FindingFixture {
	return FindingFixture{
		SchemaVersion: ReportFixtureSchemaVersion,
		Source:        source,
		RuleID:        ruleID,
		Path:          path,
		Severity:      severity,
	}
}
