package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const layerViolationRule = "architecture.layer_violation"

var evalRingPackages = map[string]string{
	"generation": "eval/internal/evalgen",
	"detection":  "eval/internal/evaldetect",
	"grading":    "eval/internal/evalgrade",
	"real-code":  "eval/internal/evalhistory",
	"tooling":    "eval/internal/evalharness",
}

// commitEvalModuleEdge commits a root module and a nested eval module whose
// package in fromRing imports the package in toRing, under this repository's
// own policy.
func commitEvalModuleEdge(fromRing, toRing string) string {
	policy, err := os.ReadFile(filepath.Join("..", "..", ".lousy-agents", "coach.json"))
	Expect(err).NotTo(HaveOccurred())

	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/fx\n\ngo 1.26.0\n")
	commitFile(repo, "eval/go.mod", "module example.com/fx/eval\n\ngo 1.26.0\n")
	commitFile(repo, coachConfigPath, string(policy))

	fromDir, toDir := evalRingPackages[fromRing], evalRingPackages[toRing]
	toPackage, fromPackage := filepath.Base(toDir), filepath.Base(fromDir)
	commitFile(repo, toDir+"/target.go", "package "+toPackage+"\n\nfunc Target() {}\n")
	commitFile(repo, fromDir+"/caller.go", "package "+fromPackage+"\n\nimport \"example.com/fx/"+toDir+"\"\n\nfunc Caller() { "+toPackage+".Target() }\n")
	return repo
}

var _ = Describe("the eval rings in this repository's policy", func() {
	DescribeTable("an import between eval rings",
		func(fromRing, toRing string, forbidden bool) {
			repo := commitEvalModuleEdge(fromRing, toRing)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", coachConfigPath, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			edge := layerViolationRule + ":" + evalRingPackages[fromRing] + "->" + evalRingPackages[toRing]
			violation := ContainElement(HaveField("Subject", edge))
			if forbidden {
				Expect(decodeCoachReport(stdout).Signals).To(violation)
			} else {
				Expect(decodeCoachReport(stdout).Signals).NotTo(violation)
			}
		},
		Entry("generation may not import detection", "generation", "detection", true),
		Entry("detection may not import generation", "detection", "generation", true),
		Entry("generation may not import grading", "generation", "grading", true),
		Entry("detection may not import grading", "detection", "grading", true),
		Entry("real-code may not import generation", "real-code", "generation", true),
		Entry("generation may not import real-code", "generation", "real-code", true),
		Entry("an eval ring may not import tooling", "grading", "tooling", true),
		Entry("grading may import detection", "grading", "detection", false),
		Entry("tooling may import generation", "tooling", "generation", false),
	)
})
