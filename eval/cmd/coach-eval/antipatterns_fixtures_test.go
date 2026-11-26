package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	antipatternCoachOnce sync.Once
	antipatternCoachPath string
)

// antipatternCoach is the coach built from this checkout, once per run.
func antipatternCoach() string {
	antipatternCoachOnce.Do(func() {
		antipatternCoachPath = filepath.Join(buildDir, "coach-antipatterns")
		build := exec.Command("go", "build", "-o", antipatternCoachPath, "./cmd/coach")
		build.Dir = repositoryRoot()
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	})
	return antipatternCoachPath
}

const (
	validMain = "package main\n\nfunc main() {}\n"
	validMod  = "module example.com/fixture\n\ngo 1.26.0\n"
)

// cannedCoach is a coach that prints the report.json in the directory it is run in, so that each variant carries the
// report the real coach would give it.
func cannedCoach() string {
	path := filepath.Join(GinkgoT().TempDir(), "coach")
	Expect(os.WriteFile(path, []byte("#!/bin/sh\ncat report.json\n"), 0o755)).To(Succeed())
	return path
}

func reportOf(signals ...string) string {
	var entries []string
	for _, signal := range signals {
		parts := strings.SplitN(signal, "|", 3)
		entries = append(entries, `{"rule_id":"`+parts[0]+`","path":"`+parts[1]+`","subject":"`+parts[2]+`"}`)
	}
	return `{"signals":[` + strings.Join(entries, ",") + `]}`
}

func variantFiles(report string, extra map[string]string) map[string]string {
	files := map[string]string{"go.mod": validMod, "main.go": validMain, "report.json": report}
	for name, content := range extra {
		files[name] = content
	}
	return files
}

// writeFixture writes fixtures/<id>/{expect.json,bad,good}; an empty expect writes no expect.json.
func writeFixture(root, id, expect string, bad, good map[string]string) {
	dir := filepath.Join(root, id)
	if expect != "" {
		Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "expect.json"), []byte(expect), 0o644)).To(Succeed())
	}
	for variant, files := range map[string]map[string]string{"bad": bad, "good": good} {
		for name, content := range files {
			target := filepath.Join(dir, variant, name)
			Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
			Expect(os.WriteFile(target, []byte(content), 0o644)).To(Succeed())
		}
	}
}

const layerExpect = `{"id":"layer","name":"Layer breach","source":"Martin","seed":"none","status":"covered",
 "expect":{"bad":[{"rule_id":"architecture.layer_violation","path":"internal/domain/order.go","subject":"architecture.layer_violation:internal/domain->internal/infra"}],"good":"none"}}`

var layerSignal = "architecture.layer_violation|internal/domain/order.go|architecture.layer_violation:internal/domain->internal/infra"

func runAntipatterns(fixtures, coach string, extraArgs ...string) generateOutcome {
	args := []string{"antipatterns", "--fixtures", fixtures}
	if coach != "" {
		args = append(args, "--coach", coach)
	}
	cmd := exec.Command(coachEvalBinary, append(args, extraArgs...)...)
	cmd.Env = os.Environ()
	return runGenerateCommand(cmd)
}

type antipatternTable struct {
	Fixtures []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Bad    struct {
			Result string `json:"result"`
			Scan   string `json:"scan"`
		} `json:"bad"`
		Good struct {
			Result string `json:"result"`
			Scan   string `json:"scan"`
		} `json:"good"`
	} `json:"fixtures"`
	Summary struct {
		Pass       int      `json:"pass"`
		Fail       int      `json:"fail"`
		NotCovered []string `json:"not_covered"`
	} `json:"summary"`
}

func decodeTable(stdout string) antipatternTable {
	var table antipatternTable
	ExpectWithOffset(1, json.Unmarshal([]byte(stdout), &table)).To(Succeed(), stdout)
	return table
}
