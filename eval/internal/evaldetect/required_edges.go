package evaldetect

import (
	"sort"
	"strings"
)

// RequiredEdgeRule is the rule ID of an import the program must make and does not.
const RequiredEdgeRule = "architecture.required_edge_absent"

// Edge is an import a program must make: a non-test Go file of the From package directory imports the To package.
type Edge struct {
	From string
	To   string
}

// RequiredEdgeAbsences reports each required edge that no non-test Go file under root makes. Imports are resolved
// against every go.mod under root as the layer graph does, and a file that does not parse is an error.
func RequiredEdgeAbsences(root string, required []Edge) ([]Finding, error) {
	modules, err := goModules(root)
	if err != nil {
		return nil, err
	}
	made := map[Edge]bool{}
	err = walkFiles(root, func(name string) bool { return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") }, func(rel string) error {
		edges, err := goFileEdges(root, rel, modules)
		for _, edge := range edges {
			made[Edge{From: edge.fromDir, To: edge.toDir}] = true
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, edge := range required {
		if !made[edge] {
			findings = append(findings, Finding{RuleID: RequiredEdgeRule, Path: edge.From, Subject: RequiredEdgeRule + ":" + edge.From + "->" + edge.To})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Subject < findings[j].Subject })
	return findings, nil
}
