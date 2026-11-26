package evalharness

import "fmt"

func scanAntipatterns(coach string, fixtures []antipattern) (antipatternTable, error) {
	table := antipatternTable{Fixtures: []fixtureReport{}, Summary: antipatternSummary{NotCovered: []string{}}}
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
		Bad: judge(fixture.Status, fixture.Expect.Bad, bad), Good: judge(fixture.Status, fixture.Expect.Good, good),
	}, nil
}

func joinVariant(dir, variant string) string { return dir + "/" + variant }

func (t *antipatternTable) add(report fixtureReport) {
	t.Fixtures = append(t.Fixtures, report)
	switch {
	case report.Status == statusNotCovered:
		t.Summary.NotCovered = append(t.Summary.NotCovered, report.ID)
	case report.Bad.Result == resultPass && report.Good.Result == resultPass:
		t.Summary.Pass++
	default:
		t.Summary.Fail++
	}
}

// failures is one line for each variant of a covered fixture that did not pass, naming the fixture, the variant and the rule.
func (t antipatternTable) failures() []string {
	var lines []string
	for _, report := range t.Fixtures {
		if report.Status == statusCovered {
			lines = append(lines, report.failureLines()...)
		}
	}
	return lines
}

func (r fixtureReport) failureLines() []string {
	var lines []string
	if r.Bad.Result != resultPass {
		lines = append(lines, fmt.Sprintf("FAIL %s bad: %s", r.ID, r.Bad.Detail))
	}
	if r.Good.Result != resultPass {
		lines = append(lines, fmt.Sprintf("FAIL %s good: %s", r.ID, r.Good.Detail))
	}
	return lines
}
