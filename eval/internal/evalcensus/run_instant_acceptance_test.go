package evalcensus_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// oneCommitRepo is a Go repository with one commit, and its SHA.
func oneCommitRepo() (string, string) {
	dir := GinkgoT().TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=f", "-c", "user.email=f@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		return strings.TrimSpace(string(output))
	}
	run("init", "-q")
	Expect(os.WriteFile(filepath.Join(dir, "p.go"), []byte("package p\n"), 0o644)).To(Succeed())
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
	return dir, run("rev-parse", "HEAD")
}

// emptyReportCoach is a coach that reports no signals.
func emptyReportCoach() string {
	path := filepath.Join(GinkgoT().TempDir(), "coach")
	Expect(os.WriteFile(path, []byte("#!/bin/sh\necho '{\"signals\":[]}'\n"), 0o755)).To(Succeed())
	return path
}

var _ = Describe("the instant a census run is recorded at", func() {
	It("dates the records and the ledger line from one reading of the clock", func() {
		repoPath, sha := oneCommitRepo()
		corpus := writeJSON(map[string]any{
			"corpus_id":    "pilot",
			"repositories": []any{map[string]any{"id": "r", "path": repoPath, "sha": sha, "language": "go"}},
			"rules":        []any{map[string]any{"rule_id": "complexity.cognitive_complexity", "languages": []any{"go"}}},
		})
		root := GinkgoT().TempDir()
		instant := time.Date(2001, 2, 3, 23, 59, 59, 999, time.UTC)
		readings := 0
		clock := func() time.Time { readings++; return instant }
		var recorded []time.Time
		line := func(_ evalcensus.Summary, _ int, at time.Time) ([]byte, error) {
			recorded = append(recorded, at)
			return []byte("{}\n"), nil
		}
		var stdout, stderr bytes.Buffer

		exit := evalcensus.Census([]string{"--coach", emptyReportCoach(), "--census-root", root, "--ledger", filepath.Join(root, "runs.jsonl"), corpus}, &stdout, &stderr, line, clock)

		Expect(exit).To(Equal(0), stderr.String())
		Expect(readings).To(Equal(1))
		Expect(recorded).To(Equal([]time.Time{instant}))
		records, err := evalcensus.LoadRecords(filepath.Join(root, "pilot", "records.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		Expect(records).NotTo(BeEmpty())
		for _, record := range records {
			Expect(record.Date).To(Equal("2001-02-03"))
		}
	})
})

var _ = Describe("who may write corpus records", func() {
	It("exports AppendRun as the only append, so every records writer holds the run-ledger lock", func() {
		fset := token.NewFileSet()
		files, err := filepath.Glob("*.go")
		Expect(err).NotTo(HaveOccurred())
		var appends []string
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, name, nil, 0)
			Expect(err).NotTo(HaveOccurred())
			for _, decl := range file.Decls {
				if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil && ast.IsExported(function.Name.Name) && strings.HasPrefix(function.Name.Name, "Append") {
					appends = append(appends, function.Name.Name)
				}
			}
		}
		Expect(appends).To(Equal([]string{"AppendRun"}))
	})
})
