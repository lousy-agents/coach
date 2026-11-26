package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const layerViolationRule = "architecture.layer_violation"

var (
	productionRings = []string{"entities", "usecases", "adapters", "app"}
	evalRings       = []string{"generation", "detection", "grading", "real-code"}

	// ringPackages names one package directory inside each ring of
	// .lousy-agents/coach.json, so an edge between rings is an import between
	// these two directories.
	ringPackages = map[string]string{
		"entities":   "pkg/domain",
		"usecases":   "pkg/usecases",
		"adapters":   "internal/adapters",
		"app":        "cmd/coach",
		"tooling":    "eval/internal/evalharness",
		"generation": "eval/internal/evalgen",
		"detection":  "eval/internal/evaldetect",
		"grading":    "eval/internal/evalgrade",
		"real-code":  "eval/internal/evalhistory",
	}

	// evalAllowedEdges are imports the ADR-007 fences leave open.
	evalAllowedEdges = [][2]string{
		{"grading", "detection"},
		{"detection", "real-code"},
		{"real-code", "detection"},
		{"tooling", "generation"},
		{"tooling", "detection"},
		{"tooling", "grading"},
		{"tooling", "real-code"},
		{"tooling", "entities"},
	}
)

// evalForbiddenEdges derives every edge the ADR-007 eval fences forbid:
// production and eval rings may not import each other, eval rings may not
// import tooling, and the eval rings fence one another.
func evalForbiddenEdges() [][2]string {
	var edges [][2]string
	for _, production := range productionRings {
		for _, eval := range evalRings {
			edges = append(edges, [2]string{production, eval}, [2]string{eval, production})
		}
	}
	for _, eval := range evalRings {
		edges = append(edges, [2]string{eval, "tooling"})
	}
	return append(edges,
		[2]string{"generation", "detection"},
		[2]string{"detection", "generation"},
		[2]string{"generation", "grading"},
		[2]string{"detection", "grading"},
		[2]string{"generation", "real-code"},
		[2]string{"real-code", "generation"},
		[2]string{"grading", "generation"},
		[2]string{"grading", "real-code"},
		[2]string{"real-code", "grading"},
	)
}

func ringEdgeEntries(edges [][2]string, forbidden bool) []TableEntry {
	verb := " may import "
	if forbidden {
		verb = " may not import "
	}
	var entries []TableEntry
	for _, edge := range edges {
		entries = append(entries, Entry(edge[0]+verb+edge[1], edge[0], edge[1], forbidden))
	}
	return entries
}

// commitRingEdge commits a root module and a nested eval module in which the
// package of fromRing imports the package of toRing, under this repository's
// own policy.
func commitRingEdge(fromRing, toRing string) string {
	policy, err := os.ReadFile(filepath.Join("..", "..", ".lousy-agents", "coach.json"))
	Expect(err).NotTo(HaveOccurred())

	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/fx\n\ngo 1.26.0\n")
	commitFile(repo, "eval/go.mod", "module example.com/fx/eval\n\ngo 1.26.0\n")
	commitFile(repo, coachConfigPath, string(policy))

	fromDir, toDir := ringPackages[fromRing], ringPackages[toRing]
	toPackage, fromPackage := filepath.Base(toDir), filepath.Base(fromDir)
	commitFile(repo, toDir+"/target.go", "package "+toPackage+"\n\nfunc Target() {}\n")
	commitFile(repo, fromDir+"/caller.go", "package "+fromPackage+"\n\nimport \""+"example.com/fx/"+toDir+"\"\n\nfunc Caller() { "+toPackage+".Target() }\n")
	return repo
}

var _ = Describe("the eval rings in this repository's policy", func() {
	entries := append(ringEdgeEntries(evalForbiddenEdges(), true), ringEdgeEntries(evalAllowedEdges, false)...)

	DescribeTable("an import across a ring boundary",
		func(fromRing, toRing string, forbidden bool) {
			repo := commitRingEdge(fromRing, toRing)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", coachConfigPath, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			edge := layerViolationRule + ":" + ringPackages[fromRing] + "->" + ringPackages[toRing]
			violation := ContainElement(HaveField("Subject", edge))
			if forbidden {
				Expect(decodeCoachReport(stdout).Signals).To(violation)
			} else {
				Expect(decodeCoachReport(stdout).Signals).NotTo(violation)
			}
		},
		entries,
	)
})
