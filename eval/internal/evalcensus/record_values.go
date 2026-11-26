package evalcensus

import "encoding/json"

// valueProblem enforces who may write which value: an automated source or
// rater writes only agree, disagree or a number; only a whole human verdict
// (human source, named human rater, a rationale) writes a verdict value.
func valueProblem(record Record) string {
	if isHumanVerdict(record) {
		return humanVerdictProblem(record)
	}
	if record.Measure == "human_verdict" {
		return "human_verdict needs a human source and a human rater"
	}
	if !isAutomatedValue(record.Value) {
		return "an automated source may write only agree, disagree or a number"
	}
	return ""
}

func isHumanVerdict(record Record) bool {
	return record.Source.Kind == "human" && record.Rater.Kind == "human"
}

func humanVerdictProblem(record Record) string {
	switch {
	case record.Measure != "human_verdict":
		return "a human rater writes only human_verdict"
	case record.Rater.ID == nil || *record.Rater.ID == "":
		return "a human verdict needs the rater's name"
	case record.Rationale == "":
		return "a human verdict needs a rationale"
	case !verdictValues[stringValue(record.Value)]:
		return "a human verdict is true_positive, false_positive, unactionable_true_positive or not_adjudicable"
	}
	return ""
}

func isAutomatedValue(value json.RawMessage) bool {
	if automatedValues[stringValue(value)] {
		return true
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return false
	}
	_, isNumber := decoded.(float64)
	return isNumber
}

// stringValue returns the value as a string, or "" when it is not one.
func stringValue(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) != nil {
		return ""
	}
	return text
}
