package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// historySource is a Go package with two functions over Coach's cognitive
// complexity threshold, F0 and F1, and three plain functions of the same
// size bin, G0 to G2, to draw controls from.
func historySource(edit string) string {
	source := complexFunctions(2)
	for g := 0; g < 3; g++ {
		source += "\nfunc G" + strconv.Itoa(g) + "() int {\n\ty := 0\n"
		for i := 0; i < 40; i++ {
			line := "\ty++\n"
			if g == 0 && i == 0 && edit != "" {
				line = edit
			}
			source += line
		}
		source += "\treturn y\n}\n"
	}
	return source
}

// historyRepo is a Go repository with a census commit and a later fix
// that touches F0 and G0, returning its path, the census SHA, the fix's
// SHA and the last SHA.
func historyRepo() (path, since, fix, until string) {
	path, since = censusRepo(goRepoFiles(historySource("")))
	edited := strings.Replace(historySource("\ty += 2\n"), "\ty := 0\n\tif x == 0 {", "\ty := 1\n\tif x == 0 {", 1)
	Expect(os.WriteFile(filepath.Join(path, "p", "p.go"), []byte(edited), 0o644)).To(Succeed())
	isolatedGit(path, "commit", "-q", "-am", "fix: F0 and G0 started from the wrong value")
	fix = isolatedGit(path, "rev-parse", "HEAD")
	Expect(os.WriteFile(filepath.Join(path, "README.md"), []byte("readme\n"), 0o644)).To(Succeed())
	isolatedGit(path, "add", "README.md")
	isolatedGit(path, "commit", "-q", "-m", "docs: add a readme")
	return path, since, fix, isolatedGit(path, "rev-parse", "HEAD")
}

type historyRepoEntry struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	SHA           string `json:"sha"`
	HistoryUntil  string `json:"history_until,omitempty"`
	Language      string `json:"language"`
	ProjectConfig string `json:"project_config,omitempty"`
}

func writeHistoryCorpus(repos ...historyRepoEntry) string {
	raw, err := json.Marshal(map[string]any{
		"corpus_id":    "fixture",
		"repositories": repos,
		"rules":        []any{map[string]any{"rule_id": "complexity.cognitive_complexity", "languages": []string{"go"}}},
	})
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(GinkgoT().TempDir(), "corpus.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

// historyOutputs names where a history run wrote its records and ledger line.
type historyOutputs struct {
	censusRoot, ledger string
}

func (o historyOutputs) records() string {
	return filepath.Join(o.censusRoot, "fixture", "records.jsonl")
}

func runHistory(env []string, corpus string, extra ...string) (compareOutcome, historyOutputs) {
	dir := GinkgoT().TempDir()
	outputs := historyOutputs{censusRoot: filepath.Join(dir, "census"), ledger: filepath.Join(dir, "ledger", "runs.jsonl")}
	args := append([]string{"history", "--coach", coachBinary(), "--census-root", outputs.censusRoot, "--ledger", outputs.ledger}, extra...)
	return runProgram("", append(os.Environ(), env...), coachEvalBinary, append(args, corpus)...), outputs
}

// gitShim puts a git first on PATH that logs each call's arguments to the
// returned file and then runs the real git.
func gitShim() (pathEnv, log string) {
	real, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())
	dir := GinkgoT().TempDir()
	log = filepath.Join(dir, "git.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	Expect(os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755)).To(Succeed())
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"), log
}

// historyRunRecord is a valid history run, as a kind: history ledger line
// carries it.
func historyRunRecord(runID string) record {
	return record{
		"schema_version":       "1",
		"run_id":               runID,
		"corpus_id":            "fixture",
		"coach":                record{"path": "coach", "sha256": strings.Repeat("ab", 32)},
		"seed":                 7,
		"controls_per_flagged": 3,
		"fix_pattern":          "fix",
		"statement":            "association, not causation",
		"repositories":         []any{},
		"exit":                 0,
	}
}
