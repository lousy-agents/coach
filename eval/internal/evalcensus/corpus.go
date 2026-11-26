package evalcensus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
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
	// Include and Exclude are repository-relative globs a finding's path must
	// pass. With no Include, every path passes the fixed exclusions and Exclude.
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
	// Licence is the licence the repository's code is published under.
	Licence string `json:"licence,omitempty"`
	// Sample is the seeded draw of each rule's findings a census takes.
	Sample *Sample `json:"sample,omitempty"`
}

// Sample is the seed of a census's draw of a rule's findings. The draw keeps SampleSize of them.
type Sample struct {
	Seed *int64 `json:"seed,omitempty"`
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

// LoadCensusCorpus reads the corpus manifest a census runs over. A census also needs each repository's licence
// and sample seed, which eval-history does not read.
func LoadCensusCorpus(path string) (Corpus, error) {
	corpus, err := LoadCorpus(path)
	if err != nil {
		return Corpus{}, err
	}
	for _, repo := range corpus.Repositories {
		if problem := censusRepositoryProblem(repo); problem != "" {
			return Corpus{}, fmt.Errorf("corpus manifest %s: repository %q: %s", path, repo.ID, problem)
		}
	}
	return corpus, nil
}

func censusRepositoryProblem(repo Repository) string {
	switch {
	case strings.TrimSpace(repo.Licence) == "":
		return "licence is empty: a census needs the licence the code is published under"
	case repo.Sample == nil || repo.Sample.Seed == nil:
		return "sample.seed is missing: a census draws its sample with a recorded seed"
	}
	return ""
}
