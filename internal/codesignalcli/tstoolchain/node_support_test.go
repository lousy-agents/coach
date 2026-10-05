package tstoolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestNodeVersionConstantsMatchDeclaredPins(t *testing.T) {
	root := coachRepoRoot(t)

	engines := packageJSONEnginesNode(t, filepath.Join(root, "js", "semantics", "package.json"))
	lockEngines := packageLockRootEnginesNode(t, filepath.Join(root, "js", "semantics", "package-lock.json"))
	if engines != lockEngines {
		t.Fatalf("js/semantics package.json engines.node = %q, package-lock.json root engines.node = %q", engines, lockEngines)
	}
	wantEnginesNode := wantEnginesNodeString(SupportedNodeMajors)
	if engines != wantEnginesNode {
		t.Fatalf("js/semantics engines.node = %q, want %q (must restate SupportedNodeMajors %v)", engines, wantEnginesNode, SupportedNodeMajors)
	}

	majors, err := nodeMajorsFromEnginesRangeUnion(engines)
	if err != nil {
		t.Fatalf("parse engines %q: %v", engines, err)
	}
	assertSameNodeMajorSet(t, "js/semantics engines.node", majors, SupportedNodeMajors)

	tested, err := testedNodeMajorFromMise(readFileT(t, filepath.Join(root, "mise.toml")))
	if err != nil {
		t.Fatalf("parse mise.toml node pin: %v", err)
	}
	if !NodeMajorSupported(tested) {
		t.Fatalf("mise.toml [tools].node pin %d is not in SupportedNodeMajors %v", tested, SupportedNodeMajors)
	}
}

func testedNodeMajorFromMise(contents string) (int, error) {
	inTools := false
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[tools]" {
			inTools = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inTools = false
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "node" {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(value), `"`)
		majorPart, _, _ := strings.Cut(raw, ".")
		return strconv.Atoi(majorPart)
	}
	return 0, strconv.ErrSyntax
}

func packageJSONEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if doc.Engines.Node == "" {
		t.Fatalf("%s engines.node is empty", path)
	}
	return doc.Engines.Node
}

func coachRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

func packageLockRootEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Packages map[string]struct {
			Engines struct {
				Node string `json:"node"`
			} `json:"engines"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	root, ok := doc.Packages[""]
	if !ok {
		t.Fatalf("%s missing packages[\"\"]", path)
	}
	if root.Engines.Node == "" {
		t.Fatalf("%s packages[\"\"].engines.node is empty", path)
	}
	return root.Engines.Node
}

// nodeMajorsFromEnginesRangeUnion parses a "^N || ^M ..." engines.node
// declaration into the set of Node majors it names. It understands only the
// caret-range-union syntax this repository's own manifests use (a discrete
// certified set, not a semver floor) -- not the full semver range grammar.
func nodeMajorsFromEnginesRangeUnion(engines string) ([]int, error) {
	terms := strings.Split(engines, "||")
	majors := make([]int, 0, len(terms))
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if !strings.HasPrefix(trimmed, "^") {
			return nil, fmt.Errorf("engines range %q: term %q is not %q-prefixed", engines, trimmed, "^")
		}
		majorPart, _, _ := strings.Cut(strings.TrimPrefix(trimmed, "^"), ".")
		major, err := strconv.Atoi(majorPart)
		if err != nil {
			return nil, fmt.Errorf("engines range %q: %w", engines, err)
		}
		majors = append(majors, major)
	}
	return majors, nil
}

func assertSameNodeMajorSet(t *testing.T, label string, got, want []int) {
	t.Helper()
	gotSorted := append([]int(nil), got...)
	wantSorted := append([]int(nil), want...)
	sort.Ints(gotSorted)
	sort.Ints(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s parses as %v, want %v", label, got, want)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s parses as %v, want %v", label, got, want)
		}
	}
}

// TestNodeMajorSupportedMatchesAnalysisGate binds CheckNode's
// set-membership predicate (NodeMajorSupported, backed by
// SupportedNodeMajors) to tsRuntime's independent AnalysisNodeMajorAllowed
// (project_ts_runtime.go): the two are frozen to agree on {24, 26} today,
// but nothing else ties them together, so a change to one that silently
// diverges from the other would make the readiness verdict and the
// analysis gate disagree on the same host Node major. This test must turn
// red the moment either one changes without the other.
func TestNodeMajorSupportedMatchesAnalysisGate(t *testing.T) {
	for major := 20; major <= 30; major++ {
		t.Run(strconv.Itoa(major), func(t *testing.T) {
			body_projectReadinessPart7Test_88(t, major)
		})
	}
}

// wantEnginesNodeString derives js/semantics' expected engines.node
// declaration directly from the compiled-in SupportedNodeMajors, so a
// legitimate future change to that constant does not require a second,
// hand-maintained restatement here to be updated in lockstep -- the
// byte-consistency check below always compares against the single source of
// truth.
func wantEnginesNodeString(majors []int) string {
	sorted := append([]int(nil), majors...)
	sort.Ints(sorted)
	terms := make([]string, len(sorted))
	for i, major := range sorted {
		terms[i] = "^" + strconv.Itoa(major)
	}
	return strings.Join(terms, " || ")
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func body_projectReadinessPart7Test_88(t *testing.T, major int) {
	if got, want := NodeMajorSupported(major), AnalysisNodeMajorAllowed(major); got != want {
		t.Fatalf("nodeMajorSupported(%d) = %t, analysisNodeMajorAllowed(%d) = %t: readiness and analysis gates disagree", major, got, major, want)
	}
}
