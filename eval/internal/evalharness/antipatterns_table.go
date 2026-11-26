package evalharness

import "fmt"

func scanAntipatterns(coach string, fixtures []antipattern) (antipatternTable, error) {
	table := antipatternTable{Fixtures: []fixtureReport{}, Summary: antipatternSummary{NotCovered: []string{}, Incomplete: []string{}}}
	for _, fixture := range fixtures {
		report, err := scanFixture(coach, fixture)
		if err != nil {
			return antipatternTable{}, err
		}
		table.add(report)
	}
	return table, nil
}

func scanFixture(coach string, fixture antipattern) (fixtureReport, error) {
	bad, err := scanVariant(coach, fixture.dir, joinVariant(fixture.dir, "bad"))
	if err != nil {
		return fixtureReport{}, err
	}
	good, err := scanVariant(coach, fixture.dir, joinVariant(fixture.dir, "good"))
	if err != nil {
		return fixtureReport{}, err
	}
	return fixtureReport{
		ID: fixture.ID, Name: fixture.Name, Source: fixture.Source, Seed: fixture.Seed, Status: fixture.Status,
		Bad: judge(fixture.Status, fixture.Expect.Bad, bad, fixture.ExpectIncomplete), Good: judge(fixture.Status, fixture.Expect.Good, good, false),
	}, nil
}

func joinVariant(dir, variant string) string { return dir + "/" + variant }

func (t *antipatternTable) add(report fixtureReport) {
	t.Fixtures = append(t.Fixtures, report)
	if report.Bad.Result == resultIncomplete || report.Good.Result == resultIncomplete {
		t.Summary.Incomplete = append(t.Summary.Incomplete, report.ID)
	}
	switch {
	case report.Status == statusNotCovered:
		t.Summary.NotCovered = append(t.Summary.NotCovered, report.ID)
		t.countFailure(report)
	case report.Bad.Result == resultPass && report.Good.Result == resultPass:
		t.Summary.Pass++
	default:
		t.Summary.Fail++
	}
}

// countFailure counts a known miss as a failure when one of its variants makes the run exit 1, such as an unexpected incomplete scan.
func (t *antipatternTable) countFailure(report fixtureReport) {
	if !report.Bad.ok || !report.Good.ok {
		t.Summary.Fail++
	}
}

// failures is one line for each variant that makes the run exit 1, naming the fixture and the variant.
func (t antipatternTable) failures() []string {
	var lines []string
	for _, report := range t.Fixtures {
		lines = append(lines, report.failureLines()...)
	}
	return lines
}

func (r fixtureReport) failureLines() []string {
	var lines []string
	if !r.Bad.ok {
		lines = append(lines, fmt.Sprintf("FAIL %s bad: %s", r.ID, r.Bad.Detail))
	}
	if !r.Good.ok {
		lines = append(lines, fmt.Sprintf("FAIL %s good: %s", r.ID, r.Good.Detail))
	}
	return lines
}
