package evalhistory

import (
	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// Statement is what every history output says about its numbers.
const Statement = "association, not causation: a flagged unit's later fix involvement compared with same-size controls"

// Summary is one history run over a corpus.
type Summary struct {
	CorpusID           string                   `json:"corpus_id"`
	Coach              evalcensus.CoachIdentity `json:"coach"`
	Seed               int64                    `json:"seed"`
	ControlsPerFlagged int                      `json:"controls_per_flagged"`
	FixPattern         string                   `json:"fix_pattern"`
	Statement          string                   `json:"statement"`
	Repositories       []RepositoryHistory      `json:"repositories"`
}

// UnmappedFixes counts the fixes whose changed lines fall in no function,
// and in no production file, as they stood at the census SHA. An
// untraceable fix counts at both levels and is listed in UnprocessedFixes.
type UnmappedFixes struct {
	Function int `json:"function"`
	File     int `json:"file"`
}

// RepositoryHistory is one repository's history between Since and Until.
type RepositoryHistory struct {
	ID                  string           `json:"id"`
	Since               string           `json:"since"`
	Until               string           `json:"until"`
	CoachExit           int              `json:"coach_exit"`
	Evidence            string           `json:"evidence,omitempty"`
	Commits             int              `json:"commits"`
	Fixes               int              `json:"fixes"`
	MatchRate           *float64         `json:"match_rate"`
	MatchRateNullReason string           `json:"match_rate_null_reason,omitempty"`
	UnmappedFixes       UnmappedFixes    `json:"unmapped_fixes"`
	UnprocessedFixes    []UnprocessedFix `json:"unprocessed_fixes"`
	Rules               []RuleResult     `json:"rules"`
	findings            []placedFinding
}

// RuleResult compares one rule's flagged units with their controls.
type RuleResult struct {
	RuleID           string   `json:"rule_id"`
	Level            Level    `json:"level"`
	Flagged          int      `json:"flagged"`
	FlaggedInvolved  int      `json:"flagged_involved"`
	Controls         int      `json:"controls"`
	ControlsInvolved int      `json:"controls_involved"`
	Shortfall        int      `json:"control_shortfall"`
	Unplaced         int      `json:"unplaced_findings"`
	UnplacedReasons  []string `json:"unplaced_reasons"`
	RiskRatio        *float64 `json:"risk_ratio"`
	Low              *float64 `json:"ci95_low"`
	High             *float64 `json:"ci95_high"`
	Reason           string   `json:"null_reason,omitempty"`
}

// AnalysisFailed says whether any repository's analysis failed.
func (summary Summary) AnalysisFailed() bool {
	for _, repo := range summary.Repositories {
		if repo.Evidence != "" {
			return true
		}
	}
	return false
}
