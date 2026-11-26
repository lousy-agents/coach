package evaldetect_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

// fakeGocognit writes a stand-in gocognit that prints stdout and exits with
// exitCode, whatever its arguments.
func fakeGocognit(exitCode int, stdout string) string {
	script := "#!/bin/sh\n"
	if stdout != "" {
		script += "cat <<'OUT'\n" + stdout + "\nOUT\n"
	}
	binary := filepath.Join(GinkgoT().TempDir(), "gocognit")
	Expect(os.WriteFile(binary, []byte(script+"exit "+itoa(exitCode)+"\n"), 0o755)).To(Succeed())
	return binary
}

func gocognitStat(root string) string {
	return `[{"PkgName":"p","FuncName":"F","Complexity":15,"Pos":{"Filename":"` + filepath.Join(root, "p", "p.go") + `","Offset":0,"Line":3,"Column":1}}]`
}

var _ = Describe("the gocognit adapter's failure handling", func() {
	When("gocognit exits 1 with output, its signal that -over matched", func() {
		It("reports the functions in that output", func() {
			root := GinkgoT().TempDir()
			gocognit := evaldetect.Gocognit{Binary: fakeGocognit(1, gocognitStat(root))}

			findings, err := gocognit.ScanWithoutVersionCheck(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", "F")))
		})
	})

	DescribeTable("a failing gocognit is an error, never a clean tree",
		func(exitCode int, withOutput bool) {
			root := GinkgoT().TempDir()
			stdout := ""
			if withOutput {
				stdout = gocognitStat(root)
			}

			_, err := evaldetect.Gocognit{Binary: fakeGocognit(exitCode, stdout)}.ScanWithoutVersionCheck(root)
			Expect(err).To(MatchError(ContainSubstring("run gocognit")))
		},
		Entry("exit 2 with output", 2, true),
		Entry("exit 1 with no output", 1, false),
	)
})

var _ = Describe("the gocognit version pin on the paths the pipeline uses", func() {
	When("Findings is given a binary of another version than its pin", func() {
		It("refuses before scanning, naming both versions", func() {
			_, err := evaldetect.Gocognit{Binary: gocognitBinary(), Version: "v1.2.0"}.Findings(GinkgoT().TempDir())

			Expect(err).To(MatchError(And(ContainSubstring("v1.2.0"), ContainSubstring(pinnedGocognit))))
		})
	})

	When("the resolver builds a binary of another version than its pin", func() {
		It("refuses, naming both versions", func() {
			_, err := evaldetect.ResolveGocognitPinnedTo(filepath.Join("..", ".."), nil, "v1.2.0")

			Expect(err).To(MatchError(And(ContainSubstring("v1.2.0"), ContainSubstring(pinnedGocognit))))
		})
	})

	When("the binary carries no gocognit module version at all", func() {
		It("refuses", func() {
			goBinary, err := exec.LookPath("go")
			Expect(err).NotTo(HaveOccurred())

			Expect(evaldetect.Gocognit{Binary: goBinary, Version: pinnedGocognit}.CheckVersion()).
				To(MatchError(ContainSubstring("carries no")))
		})
	})
})
