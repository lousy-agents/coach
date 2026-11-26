package evalcensus

import (
	"fmt"
	"regexp"
	"time"
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// validateRecord checks one record's fields in isolation.
func validateRecord(record Record) error {
	if problem := recordShapeProblem(record); problem != "" {
		return fmt.Errorf("%w %q: %s", ErrInvalidRecord, record.RecordID, problem)
	}
	if problem := valueProblem(record); problem != "" {
		return fmt.Errorf("%w %q: %s", ErrInvalidRecord, record.RecordID, problem)
	}
	return nil
}

func recordShapeProblem(record Record) string {
	switch {
	case record.RecordID == "":
		return "record_id is empty"
	case targetProblem(record.Target) != "":
		return targetProblem(record.Target)
	case !measures[record.Measure]:
		return "unknown measure " + record.Measure
	case !sourceKinds[record.Source.Kind]:
		return "unknown source kind " + record.Source.Kind
	case !raterKinds[record.Rater.Kind]:
		return "unknown rater kind " + record.Rater.Kind
	case !isDate(record.Date):
		return "date is not YYYY-MM-DD"
	case record.Disposition != nil && !dispositions[*record.Disposition]:
		return "unknown disposition " + *record.Disposition
	case record.EngineerAction != nil && !engineerActions[*record.EngineerAction]:
		return "unknown engineer_action " + *record.EngineerAction
	}
	return ""
}

func targetProblem(target Target) string {
	switch {
	case target.CorpusID == "":
		return "target.corpus_id is empty"
	case !shaPattern.MatchString(target.SHA):
		return "target.sha is not 40 lowercase hex digits"
	case target.RuleID == "":
		return "target.rule_id is empty"
	case target.Path == "":
		return "target.path is empty"
	case target.Lines[0] < 1 || target.Lines[1] < target.Lines[0]:
		return "target.lines is not a range starting at 1 or later"
	}
	return ""
}

func isDate(text string) bool {
	_, err := time.Parse(time.DateOnly, text)
	return err == nil
}
