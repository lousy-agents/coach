package evalcensus

import (
	"encoding/json"
	"errors"
)

// ErrInvalidRecord marks a census record the ledger refuses.
var ErrInvalidRecord = errors.New("evalcensus: invalid record")

// Record is one census record (docs/evaluation/agentic-eval-design.md §11).
type Record struct {
	RecordID       string          `json:"record_id"`
	Target         Target          `json:"target"`
	Measure        string          `json:"measure"`
	Value          json.RawMessage `json:"value"`
	Source         Source          `json:"source"`
	Rater          Rater           `json:"rater"`
	Date           string          `json:"date"`
	Rationale      string          `json:"rationale"`
	Disposition    *string         `json:"disposition"`
	EngineerAction *string         `json:"engineer_action"`
	Supersedes     *string         `json:"supersedes"`
}

// Target names the finding a record is about. Lines is nil for a
// file-level finding, which Coach reports with no line span.
type Target struct {
	CorpusID    string  `json:"corpus_id"`
	SHA         string  `json:"sha"`
	RuleID      string  `json:"rule_id"`
	Path        string  `json:"path"`
	Subject     *string `json:"subject"`
	Lines       *[2]int `json:"lines"`
	Fingerprint *string `json:"fingerprint"`
}

// Source is where a record's value came from.
type Source struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// Rater is who or what assigned the value.
type Rater struct {
	ID   *string `json:"id"`
	Kind string  `json:"kind"`
}

var (
	measures        = setOf("detector_agreement", "flagged_vs_unflagged_association", "human_verdict", "analysis_status")
	sourceKinds     = setOf("detector", "szz", "review_comment", "follow_up_fix", "human", "census")
	raterKinds      = setOf("automated", "human")
	dispositions    = setOf("retune", "rescope", "default_off", "accept_with_caveat")
	engineerActions = setOf("acted", "deferred", "abandoned", "no_usage_observed")
	automatedValues = setOf("agree", "disagree")
	statusValues    = setOf(string(Measured), string(NotExercised), string(NotAdjudicableOnly), string(AnalysisFailed), string(OutOfScope))
	verdictValues   = setOf("true_positive", "false_positive", "unactionable_true_positive", "not_adjudicable")
)

func setOf(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
