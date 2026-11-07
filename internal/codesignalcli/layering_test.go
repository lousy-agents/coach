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
	"":                    {"gitrepo", "pkgmanager", "policyref", "projectcheck", "projectconfig", "projectreadiness", "revisionfs", "tstoolchain"},
	"configauthoring":     {"gitrepo", "projectconfig", "prompt", "revisionfs"},
	"gitrepo":             {"shellword"},
	"internal/gitfixture": {},
	"invocation":          {"policyref", "shellword"},
	"pkgmanager":          {"gitrepo", "projectreadiness", "subprocess", "tstoolchain"},
	"policyref":           {},
	"projectcheck":        {"gitrepo", "pkgmanager", "policyref", "projectconfig", "projectreadiness", "tstoolchain"},
	"projectconfig":       {"gitrepo", "projectreadiness", "shellword"},
	"projectreadiness":    {},
	"prompt":              {},
	"render":              {"projectreadiness", "shellword"},
	"revisionfs":          {"gitrepo"},
	"shellword":           {},
	"sourcescope":         {"gitrepo"},
	"subprocess":          {},
	"terminal":            {},
	"tssetup":             {"gitrepo", "invocation", "pkgmanager", "policyref", "projectcheck", "projectconfig", "projectreadiness", "prompt", "tstoolchain"},
	"tstoolchain":         {"gitrepo", "invocation", "policyref", "projectreadiness", "subprocess"},
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

// TestPolicyrefImportsOnlyTheStandardLibrary pins policyref as a leaf. An empty
// row in allowedSiblingImports keeps it from importing a sibling, but a value
// type that the phrase builder and the toolchain errors depend on must not pull
// in anything else of this module or of a third party either, or those packages
// inherit the coupling the leaf exists to avoid.
func TestPolicyrefImportsOnlyTheStandardLibrary(t *testing.T) {
	output, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, "./policyref").Output()
	if err != nil {
		t.Fatalf("go list ./policyref: %v", err)
	}
	for _, imported := range strings.Fields(string(output)) {
		if firstElement, _, _ := strings.Cut(imported, "/"); strings.Contains(firstElement, ".") {
			t.Errorf("policyref imports %s: only the standard library is allowed", imported)
		}
	}
}
