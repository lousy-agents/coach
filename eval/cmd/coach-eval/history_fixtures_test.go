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
		first := "\ty++\n"
		if g == 0 && edit != "" {
			first = edit
		}
		source += "\nfunc G" + strconv.Itoa(g) + "() int {\n\ty := 0\n" + first + strings.Repeat("\ty++\n", 39) + "\treturn y\n}\n"
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

// writeHistoryCorpus writes a manifest that both a census and a history can read: the licence and sample seed are
// the census's, and history ignores them.
func writeHistoryCorpus(repos ...historyRepoEntry) string {
	entries := make([]map[string]any, 0, len(repos))
	for _, repo := range repos {
		raw, err := json.Marshal(repo)
		Expect(err).NotTo(HaveOccurred())
		var entry map[string]any
		Expect(json.Unmarshal(raw, &entry)).To(Succeed())
		entry["licence"] = "MIT"
		entry["sample"] = map[string]any{"seed": 7}
		entries = append(entries, entry)
	}
	raw, err := json.Marshal(map[string]any{
		"corpus_id":    "fixture",
		"repositories": entries,
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

// historyArgsWithout is a history command line with each flag in flags,
// leaving out drop, or with the manifest twice when drop is "one manifest".
func historyArgsWithout(drop string, flags map[string]string, corpus string) []string {
	args := []string{"history", "--seed", "7"}
	for _, flag := range []string{"--coach", "--census-root", "--ledger"} {
		if flag != drop {
			args = append(args, flag, flags[flag])
		}
	}
	args = append(args, corpus)
	if drop == "one manifest" {
		args = append(args, corpus)
	}
	return args
}

// expectOnlyLocalGit fails on any logged git call that reaches a network:
// a fetch, pull, push, remote listing, a URL, or a clone of anything but
// the local directory.
func expectOnlyLocalGit(calls []string, localPath string) {
	for _, call := range calls {
		Expect(call).NotTo(MatchRegexp(`\b(fetch|pull|push|ls-remote|remote-https?)\b`), call)
		Expect(call).NotTo(ContainSubstring("://"), call)
		if strings.Contains(call, " clone ") || strings.HasPrefix(call, "clone ") {
			Expect(call).To(ContainSubstring(localPath), call)
		}
	}
}
