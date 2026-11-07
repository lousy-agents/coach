//go:build unix

package projectmodel_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// goThatRecordsEveryRun puts a go on PATH that appends its arguments and the toolchain setting it saw to the
// file it returns, then runs the go that was on PATH before it.
func goThatRecordsEveryRun() string {
	realGo, err := exec.LookPath("go")
	Expect(err).NotTo(HaveOccurred())
	record := filepath.Join(GinkgoT().TempDir(), "record")
	bin := GinkgoT().TempDir()
	script := "#!/bin/sh\nprintf '%s\\ttoolchain=%s\\n' \"$*\" \"$GOTOOLCHAIN\" >> '" + record + "'\nexec '" + realGo + "' \"$@\"\n"
	Expect(os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755)).To(Succeed())
	GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

// The packages of a Go project are loaded from a directory of committed bytes. go stamps version control
// information into what it lists unless told not to, which runs git in that directory, and a go.mod there can name
// a toolchain for go to fetch and run. Neither may be left to what the caller's environment happens to say.
var _ = Describe("BuildGoCallGraph loads the project's packages without trusting its directory", func() {
	It("runs the go list of the project's packages without version control stamping, and every go with the installed toolchain", func() {
		record := goThatRecordsEveryRun()
		GinkgoT().Setenv("GOTOOLCHAIN", "auto")
		GinkgoT().Setenv("GOFLAGS", "-buildvcs=true")

		_, err := projectmodel.BuildGoCallGraph(context.Background(), os.DirFS("testdata/go_callgraph_direct"), projectmodel.CallGraphOptions{})
		Expect(err).NotTo(HaveOccurred())

		ran, err := os.ReadFile(record)
		Expect(err).NotTo(HaveOccurred(), "go was not run")
		runs := strings.Split(strings.TrimSpace(string(ran)), "\n")
		Expect(runs).To(ContainElement(ContainSubstring(" -- ./...\t")), "the packages of the project were not listed:\n%s", ran)
		for _, run := range runs {
			Expect(run).To(HaveSuffix("\ttoolchain=local"), "a go ran with the caller's toolchain setting:\n%s", ran)
			if strings.Contains(run, " -- ./...\t") {
				Expect(run).To(ContainSubstring("-buildvcs=false"), "the go list of the project's packages stamped version control information:\n%s", ran)
			}
		}
	})
})
