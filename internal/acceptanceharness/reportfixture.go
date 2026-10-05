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
