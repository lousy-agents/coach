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

// A policy is judged on disk, so no sentence the production code of the package prints says the policy must be committed
// to be used. The unusable-policy step and the refusal of compiler setup are the two places that said it.
func TestNoProductionSentenceSaysAPolicyMustBeCommittedToBeUsed(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, sentence := range []string{"the committed policy cannot be used", "a reviewed, committed policy is required"} {
			if strings.Contains(string(source), sentence) {
				t.Errorf("%s still says %q", file, sentence)
			}
		}
	}
}

func TestTheUnusablePolicyStepIsDefinedInOneProductionFile(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var defining []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "the policy on disk cannot be used: fix the file") {
			defining = append(defining, file)
		}
	}
	if len(defining) != 1 {
		t.Errorf("the unusable-policy sentence is in %v; want exactly one production file", defining)
	}
}

func TestTheScanAndThePrepareCompilerRefusalSayTheSameSentence(t *testing.T) {
	invalid := configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor)

	scan := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", false, "")
	refusal := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyAuthor, "", "abc123", "pol.json")

	if !strings.Contains(scan, unusablePolicyStep) || !strings.Contains(refusal, unusablePolicyStep) {
		t.Errorf("scan line %q and refusal %q; want both to carry %q", scan, refusal, unusablePolicyStep)
	}
}
