package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// pathExcludingExecutables returns the current process's PATH with every
// directory that contains any of names removed, used below to prove the
// core `coach codesignal` path (no --project-config, i.e. pkg/semantics
// only) never spawns or depends on Node -- a headline architectural
// guarantee of issue #214: pkg/projectmodel's real TypeScript sidecar
// wiring (pkg/projectmodel/ts_sidecar.go) is opt-in, and the default Go/
// CodeSignal pipeline must keep working in a Go-only environment with no
// Node installation at all.
func pathExcludingExecutables(names ...string) string {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		excluded := false
		for _, name := range names {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
				excluded = true
				break
			}
		}
		if !excluded {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

var _ = Describe("coach codesignal core Go path with Node absent from PATH", func() {
	When("no directory on the child process's PATH contains a node or npm executable", func() {
		It("still completes a Repository Baseline scan successfully", func() {
			strippedPath := pathExcludingExecutables("node", "npm")

			// Belt-and-suspenders: prove node/npm are genuinely unreachable under
			// strippedPath, so a passing scan below cannot be a false green
			// caused by node/npm still being reachable some other way.
			probe := exec.Command("sh", "-c", "command -v node || command -v npm")
			probe.Env = []string{"PATH=" + strippedPath}
			Expect(probe.Run()).To(HaveOccurred(), "expected neither node nor npm to be found on the stripped PATH used for this spec")

			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			headSHA := commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			command := exec.Command(commandPath, "codesignal", "--baseline", "--format=json")
			command.Dir = repo
			command.Env = []string{"PATH=" + strippedPath, "HOME=" + os.Getenv("HOME")}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			Expect(command.Run()).To(Succeed(), "stderr: %s", stderr.String())

			var report codesignal.Report
			Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout.String())
			Expect(report.Scope.Baseline).To(BeTrue())
			Expect(report.Scope.Revision).To(Equal(headSHA))
		})
	})
})
