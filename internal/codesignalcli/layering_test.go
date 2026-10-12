package codesignalcli

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePrefix = "github.com/lousy-agents/coach/internal/codesignalcli"

// allowedSiblingImports is the package DAG ADR-007 asks for: adapters and
// the readiness model at the bottom, use cases above them, and the scan
// use case in the root. A package may import only the siblings listed for
// it, so a new edge has to be added here deliberately.
var allowedSiblingImports = map[string][]string{
	"":                    {"gitrepo", "pkgmanager", "projectcheck", "projectconfig", "projectreadiness", "revisionfs", "tstoolchain"},
	"configauthoring":     {"gitrepo", "projectconfig", "prompt", "revisionfs"},
	"gitrepo":             {},
	"internal/gitfixture": {},
	"pkgmanager":          {"gitrepo", "projectreadiness", "subprocess", "tstoolchain"},
	"projectcheck":        {"gitrepo", "pkgmanager", "projectconfig", "projectreadiness", "tstoolchain"},
	"projectconfig":       {"gitrepo"},
	"projectreadiness":    {},
	"prompt":              {},
	"render":              {"projectreadiness"},
	"revisionfs":          {"gitrepo"},
	"sourcescope":         {"gitrepo"},
	"subprocess":          {},
	"terminal":            {},
	"tssetup":             {"gitrepo", "pkgmanager", "projectcheck", "projectconfig", "projectreadiness", "prompt", "tstoolchain"},
	"tstoolchain":         {"gitrepo", "projectreadiness", "subprocess"},
}

func TestSubPackageImportsFollowTheLayering(t *testing.T) {
	output, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list ./...: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		pkg := siblingName(fields[0])
		allowed, known := allowedSiblingImports[pkg]
		if !known {
			t.Errorf("package %q has no entry in allowedSiblingImports: decide where it sits in the layering", fields[0])
			continue
		}
		for _, sibling := range disallowedSiblings(allowed, fields[1:]) {
			t.Errorf("%s imports %s, which allowedSiblingImports does not permit", fields[0], modulePrefix+"/"+sibling)
		}
	}
}

func disallowedSiblings(allowed, imports []string) []string {
	var disallowed []string
	for _, sibling := range siblingImports(imports) {
		if !slices.Contains(allowed, sibling) {
			disallowed = append(disallowed, sibling)
		}
	}
	return disallowed
}

func siblingName(importPath string) string {
	return strings.TrimPrefix(strings.TrimPrefix(importPath, modulePrefix), "/")
}

func siblingImports(imports []string) []string {
	var siblings []string
	for _, imported := range imports {
		if imported == modulePrefix || strings.HasPrefix(imported, modulePrefix+"/") {
			siblings = append(siblings, siblingName(imported))
		}
	}
	return siblings
}
