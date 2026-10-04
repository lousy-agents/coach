package acceptanceharness

// NewReportFixture builds a ReportFixture stamped with the current
// ReportFixtureSchemaVersion, so callers can't accidentally omit or
// mismatch it.
func NewReportFixture(findings []FindingFixture) ReportFixture {
	return ReportFixture{
		SchemaVersion: ReportFixtureSchemaVersion,
		Findings:      findings,
	}
}
