package evalcensus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// Corpus is the pinned real corpus a census runs over: each repository at a
// SHA, and the rules censused with the languages each applies to.
type Corpus struct {
	ID           string       `json:"corpus_id"`
	Repositories []Repository `json:"repositories"`
	Rules        []Rule       `json:"rules"`
}

// Repository is one corpus repository, read from a local clone.
type Repository struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	SHA           string `json:"sha"`
	Language      string `json:"language"`
	ProjectConfig string `json:"project_config"`
	// HistoryUntil is the last commit a history run reads fixes up to. A
	// census does not use it.
	HistoryUntil string `json:"history_until,omitempty"`
}

// Rule is one censused rule and the languages it applies to.
type Rule struct {
	RuleID    string   `json:"rule_id"`
	Languages []string `json:"languages"`
}

var (
	corpusIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	languages       = setOf("go", "typescript")
)

func LoadCorpus(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, fmt.Errorf("read corpus manifest: %w", err)
	}
	var corpus Corpus
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return Corpus{}, fmt.Errorf("parse corpus manifest %s: %w", path, err)
	}
	if problem := corpusProblem(corpus); problem != "" {
		return Corpus{}, fmt.Errorf("corpus manifest %s: %s", path, problem)
	}
	return corpus, nil
}

func corpusProblem(corpus Corpus) string {
	switch {
	case !corpusIDPattern.MatchString(corpus.ID):
		return "corpus_id must be a lowercase path segment"
	case len(corpus.Repositories) == 0:
		return "repositories is empty"
	case len(corpus.Rules) == 0:
		return "rules is empty"
	}
	if problem := repositoriesProblem(corpus.Repositories); problem != "" {
		return problem
	}
	return rulesProblem(corpus.Rules)
}
