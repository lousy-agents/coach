package acceptanceharness

// ReportFixtureSchemaVersion is the current version of the report-fixture
// schema this package defines for golden report/finding fixtures. This is a
// separate, independently-versioned vocabulary from FixtureSchemaVersion
// (which pins Task 0.1's GitHub-fixture/request-record schema): a report
// fixture evolves on its own timeline as findings gain new provenance or
// shape, and must not be forced to share a version counter with an
// unrelated fixture kind. Per docs/architecture/acceptance-harness.md
// section 3, every golden fixture embeds an explicit schema/version
// identifier so additive report evolution doesn't invalidate or silently
// reinterpret an older golden.
const ReportFixtureSchemaVersion = 1

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

// ReportFixture is the top-level envelope a golden report fixture embeds:
// a versioned list of findings, additive across schema versions (a later
// version may add findings of a new FindingSource without invalidating or
// requiring edits to an earlier golden's file).
type ReportFixture struct {
	SchemaVersion int              `json:"schema_version"`
	Findings      []FindingFixture `json:"findings"`
}

// NewReportFixture builds a ReportFixture stamped with the current
// ReportFixtureSchemaVersion, so callers can't accidentally omit or
// mismatch it.
func NewReportFixture(findings []FindingFixture) ReportFixture {
	return ReportFixture{
		SchemaVersion: ReportFixtureSchemaVersion,
		Findings:      findings,
	}
}
