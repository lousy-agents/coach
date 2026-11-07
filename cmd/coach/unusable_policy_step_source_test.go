package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The sentence that says how to get past a policy that exists and cannot be used is printed by the scan and by the
// --prepare-compiler refusal. It lives in one constant, so the two cannot say different things; this holds the production
// code of the package to having it written once.
func TestTheSentenceForAnUnusablePolicyIsWrittenOnceInTheProductionCode(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	written := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		written += strings.Count(string(source), "the committed policy cannot be used")
	}
	if written != 1 {
		t.Errorf("the sentence is written %d times in the non-test files of cmd/coach; want once, in unusablePolicyStep", written)
	}
}

func TestTheScanAndThePrepareCompilerRefusalSayTheSameSentence(t *testing.T) {
	invalid := configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor)

	scan := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", false, "")
	refusal := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyAuthor, "abc123", "pol.json")

	if !strings.Contains(scan, unusablePolicyStep) || !strings.Contains(refusal, unusablePolicyStep) {
		t.Errorf("scan line %q and refusal %q; want both to carry %q", scan, refusal, unusablePolicyStep)
	}
}
