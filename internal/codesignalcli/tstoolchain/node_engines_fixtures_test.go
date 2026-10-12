package tstoolchain

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

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
