package evalcensus

import (
	"encoding/json"
	"slices"
)

// Status is a rule's analysis_status on one repository.
type Status string

const (
	Measured           Status = "measured"
	NotExercised       Status = "not_exercised"
	NotAdjudicableOnly Status = "not_adjudicable_only"
	AnalysisFailed     Status = "analysis_failed"
	OutOfScope         Status = "out_of_scope"
)

// RuleStatus is one rule's census outcome on one repository. Adjudicable
// counts the findings that carry evidence a reviewer can judge.
type RuleStatus struct {
	RuleID      string `json:"rule_id"`
	Status      Status `json:"analysis_status"`
	Findings    int    `json:"findings"`
	Adjudicable int    `json:"adjudicable"`
	Evidence    string `json:"evidence,omitempty"`
}

type reportSignal struct {
	RuleID   string `json:"rule_id"`
	Evidence string `json:"evidence"`
}

// StatusesFromReport derives each corpus rule's status from Coach's JSON
// report on repo. A report that cannot be read is an analysis failure.
func StatusesFromReport(corpus Corpus, repo Repository, report []byte) []RuleStatus {
	var decoded struct {
		Signals *[]reportSignal `json:"signals"`
	}
	if err := json.Unmarshal(report, &decoded); err != nil || decoded.Signals == nil {
		return StatusesFromFailure(corpus, repo, "Coach's report could not be read as a JSON report with signals")
	}
	statuses := make([]RuleStatus, 0, len(corpus.Rules))
	for _, rule := range corpus.Rules {
		statuses = append(statuses, ruleStatus(rule, repo, *decoded.Signals))
	}
	return statuses
}

func ruleStatus(rule Rule, repo Repository, signals []reportSignal) RuleStatus {
	status := RuleStatus{RuleID: rule.RuleID}
	if !slices.Contains(rule.Languages, repo.Language) {
		status.Status = OutOfScope
		return status
	}
	for _, signal := range signals {
		if signal.RuleID != rule.RuleID {
			continue
		}
		status.Findings++
		if signal.Evidence != "" {
			status.Adjudicable++
		}
	}
	status.Status = countedStatus(status)
	return status
}

func countedStatus(status RuleStatus) Status {
	switch {
	case status.Findings == 0:
		return NotExercised
	case status.Adjudicable == 0:
		return NotAdjudicableOnly
	}
	return Measured
}

// StatusesFromFailure records analysis_failed, with the evidence, for every
// corpus rule that applies to repo's language.
func StatusesFromFailure(corpus Corpus, repo Repository, evidence string) []RuleStatus {
	statuses := make([]RuleStatus, 0, len(corpus.Rules))
	for _, rule := range corpus.Rules {
		status := RuleStatus{RuleID: rule.RuleID, Status: OutOfScope}
		if slices.Contains(rule.Languages, repo.Language) {
			status.Status, status.Evidence = AnalysisFailed, evidence
		}
		statuses = append(statuses, status)
	}
	return statuses
}
