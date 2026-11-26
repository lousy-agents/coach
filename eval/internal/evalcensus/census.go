package evalcensus

import (
	"encoding/json"
	"errors"
)

var errNotImplemented = errors.New("evalcensus: not implemented")

// ErrInvalidRecord marks a census record the ledger refuses.
var ErrInvalidRecord = errors.New("evalcensus: invalid record")

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

type Target struct {
	CorpusID    string  `json:"corpus_id"`
	SHA         string  `json:"sha"`
	RuleID      string  `json:"rule_id"`
	Path        string  `json:"path"`
	Subject     *string `json:"subject"`
	Lines       [2]int  `json:"lines"`
	Fingerprint *string `json:"fingerprint"`
}

type Source struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type Rater struct {
	ID   *string `json:"id"`
	Kind string  `json:"kind"`
}

func AppendRecords(path string, records []Record) error { return errNotImplemented }

func LoadRecords(path string) ([]Record, error) { return nil, errNotImplemented }

type Corpus struct {
	ID           string
	Repositories []Repository
	Rules        []Rule
}

type Repository struct {
	ID, Path, SHA, Language, ProjectConfig string
}

type Rule struct {
	RuleID    string
	Languages []string
}

func LoadCorpus(path string) (Corpus, error) { return Corpus{}, errNotImplemented }

type Status string

const (
	Measured           Status = "measured"
	NotExercised       Status = "not_exercised"
	NotAdjudicableOnly Status = "not_adjudicable_only"
	AnalysisFailed     Status = "analysis_failed"
	OutOfScope         Status = "out_of_scope"
)

type RuleStatus struct {
	RuleID      string
	Status      Status
	Findings    int
	Adjudicable int
	Evidence    string
}

func StatusesFromReport(corpus Corpus, repo Repository, report []byte) []RuleStatus { return nil }

func StatusesFromFailure(corpus Corpus, repo Repository, evidence string) []RuleStatus { return nil }
