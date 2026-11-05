package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestPolicyRequiredLineNamesOnlyTheRemedyReadinessNamed(t *testing.T) {
	cases := []struct {
		name    string
		remedy  projectreadiness.PolicyRemedy
		path    string
		want    []string
		notWant []string
	}{
		{"a committed policy is restored", projectreadiness.RemedyRestore, "pol.json",
			[]string{"(policy_missing)", "restore_policy", "git restore --source=abc123 --staged --worktree -- ':(top,literal)pol.json'"}, []string{"author_policy", "guided policy authoring"}},
		{"a committed policy whose name cannot be printed is restored without a command", projectreadiness.RemedyRestore, "bad\x1bname.json",
			[]string{"restore_policy", "from git"}, []string{"git restore", "\x1b", "author_policy"}},
		{"a policy that was never committed is authored", projectreadiness.RemedyAuthor, "pol.json",
			[]string{"author_policy", "guided policy authoring"}, []string{"restore_policy", "git restore"}},
		{"a policy a new file cannot fix names neither", projectreadiness.RemedyNone, "pol.json",
			[]string{"(policy_missing)", "resolve the policy path first"}, []string{"restore_policy", "author_policy", "git restore"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertLineMentions(t, policyRequiredLine("coach codesignal", "policy_missing", c.remedy, "abc123", c.path), c.want, c.notWant)
		})
	}
}

func TestPolicyRequiredLineEndsWithTheRestoreCommandSoItPastesAsWritten(t *testing.T) {
	line := policyRequiredLine("coach codesignal", "policy_missing", projectreadiness.RemedyRestore, "abc123", "pol.json")

	if want := "-- ':(top,literal)pol.json'"; !strings.HasSuffix(line, want) {
		t.Errorf("line %q does not end with the command %q: text after it joins the pathspec when the line is copied", line, want)
	}
}

func TestPolicyRequiredLineSaysWhyAnUnprintableNameHasNoCommandAsTheScanErrorDoes(t *testing.T) {
	line := policyRequiredLine("coach codesignal", "policy_missing", projectreadiness.RemedyRestore, "abc123", "bad\x1bname.json")

	if want := "the name has a character that is not printed here"; !strings.Contains(line, want) {
		t.Errorf("line %q does not say why no command is shown (%q)", line, want)
	}
}

func assertLineMentions(t *testing.T, line string, want, notWant []string) {
	t.Helper()
	for _, text := range want {
		if !strings.Contains(line, text) {
			t.Errorf("line %q does not contain %q", line, text)
		}
	}
	for _, text := range notWant {
		if strings.Contains(line, text) {
			t.Errorf("line %q contains %q", line, text)
		}
	}
}
