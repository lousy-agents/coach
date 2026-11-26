package evalhistory_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// unitsSource has A on lines 3-5 and B, C, D, E on four lines each, at
// 7-10, 12-15, 17-20 and 22-25.
const unitsSource = "package p\n\nfunc A() int {\n\treturn 1\n}\n\n" +
	"func B() int {\n\tx := 2\n\treturn x\n}\n\n" +
	"func C() int {\n\tx := 3\n\treturn x\n}\n\n" +
	"func D() int {\n\tx := 4\n\treturn x\n}\n\n" +
	"func E() int {\n\tx := 5\n\treturn x\n}\n"

const varSource = "package p\n\nvar V = 1\n"

// reportSignal is the part of a Coach JSON signal the history reads.
type reportSignal struct {
	RuleID      string         `json:"rule_id"`
	Path        string         `json:"path"`
	Subject     string         `json:"subject,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	Location    map[string]int `json:"location"`
}

func functionSignal(ruleID, path, subject string, startRow, endRow int) reportSignal {
	return reportSignal{RuleID: ruleID, Path: path, Subject: subject, Fingerprint: "fp-" + subject, Location: map[string]int{"start_row": startRow, "end_row": endRow}}
}

func fileSignal(ruleID, path string) reportSignal {
	return reportSignal{RuleID: ruleID, Path: path, Fingerprint: "fp-" + path, Location: map[string]int{"start_row": 0, "end_row": 0}}
}

// fakeCoach writes an executable that prints a Coach JSON report with
// signals, or fails with exit 1 when signals is nil.
func fakeCoach(signals []reportSignal) string {
	dir := GinkgoT().TempDir()
	script := "#!/bin/sh\necho 'coach: scan failed' >&2\nexit 1\n"
	if signals != nil {
		report, err := json.Marshal(map[string]any{"signals": signals})
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "report.json"), report, 0o644)).To(Succeed())
		script = "#!/bin/sh\ncat '" + filepath.Join(dir, "report.json") + "'\n"
	}
	path := filepath.Join(dir, "coach")
	Expect(os.WriteFile(path, []byte(script), 0o755)).To(Succeed())
	return path
}

// corpusFor is a one-repository Go corpus censusing rules over [since, until].
func corpusFor(h history, since, until string, rules ...evalcensus.Rule) evalcensus.Corpus {
	return evalcensus.Corpus{
		ID:           "history",
		Repositories: []evalcensus.Repository{{ID: "r", Path: h.dir, SHA: since, HistoryUntil: until, Language: "go"}},
		Rules:        rules,
	}
}

func goRule(ruleID string) evalcensus.Rule {
	return evalcensus.Rule{RuleID: ruleID, Languages: []string{"go"}}
}

// fakeCoachPrinting writes an executable that prints output and exits 0.
func fakeCoachPrinting(output string) string {
	dir := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(dir, "out"), []byte(output), 0o644)).To(Succeed())
	path := filepath.Join(dir, "coach")
	Expect(os.WriteFile(path, []byte("#!/bin/sh\ncat '"+filepath.Join(dir, "out")+"'\n"), 0o755)).To(Succeed())
	return path
}
