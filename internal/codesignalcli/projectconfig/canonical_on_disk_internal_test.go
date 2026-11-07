package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Discovery asks the working tree about the default policy's path the way a named policy is asked, so a path that
// cannot be the policy is told apart from one that is only not committed, and from nothing at all. The scan
// never reads the working tree for a discovered policy; what it reports about the path is a diagnostic.
func TestDiscoveryNamesWhatTheDiskHoldsAtTheDefaultPolicyPath(t *testing.T) {
	cases := map[string]struct {
		arrange       func(t *testing.T, repo string)
		uncommitted   bool
		unusableCause projectreadiness.PolicyCause
	}{
		"nothing there":                        {func(*testing.T, string) {}, false, ""},
		"a regular file":                       {writeCanonical, true, ""},
		"a directory":                          {makeCanonicalDirectory, false, projectreadiness.CauseNotARegularFile},
		"a symbolic link":                      {linkCanonical, false, projectreadiness.CauseSymbolicLink},
		"a file where the directory should be": {fileInPlaceOfTheDirectory, false, projectreadiness.CauseParentNotDirectory},
		"a parent the process cannot search":   {unsearchableParent, false, projectreadiness.CauseUnreadableOnDisk},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireWhatDiscoverySaysOfTheDisk(t, c.arrange, c.uncommitted, c.unusableCause) })
	}
}

func requireWhatDiscoverySaysOfTheDisk(t *testing.T, arrange func(*testing.T, string), uncommitted bool, unusableCause projectreadiness.PolicyCause) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	arrange(t, repo)

	selection, err := Discover(repo, "HEAD")

	if err != nil || selection.Fatal != nil {
		t.Fatalf("Discover = %+v, %v; want a scan that goes on without a policy", selection, err)
	}
	if selection.UncommittedCanonical != uncommitted {
		t.Errorf("UncommittedCanonical = %v, want %v", selection.UncommittedCanonical, uncommitted)
	}
	if got := causeOf(selection.UnusableCanonical); got != unusableCause {
		t.Errorf("UnusableCanonical cause = %q, want %q", got, unusableCause)
	}
}

func causeOf(refusal *ConfigError) projectreadiness.PolicyCause {
	if refusal == nil {
		return ""
	}
	return refusal.PolicyCause
}

// The diagnostic names the cause the project check reports for the same path, in the form its text prints it,
// and says the scan did not read the file.
func TestTheUnusableCanonicalMessageNamesTheCauseAndSaysTheScanDidNotReadIt(t *testing.T) {
	for _, cause := range []projectreadiness.PolicyCause{
		projectreadiness.CauseNotARegularFile, projectreadiness.CauseSymbolicLink, projectreadiness.CauseUnreadableOnDisk,
		projectreadiness.CauseParentNotDirectory, projectreadiness.CauseNameCaseDiffers,
	} {
		t.Run(string(cause), func(t *testing.T) { requireTheMessageNamesTheCause(t, cause) })
	}
}

func requireTheMessageNamesTheCause(t *testing.T, cause projectreadiness.PolicyCause) {
	t.Helper()
	message := UnusableCanonicalMessage(&ConfigError{PolicyCause: cause})

	if !strings.Contains(message, "(cause="+string(cause)+")") || !strings.Contains(message, "so this scan did not read it") || strings.Contains(message, "the working tree cannot use it") {
		t.Errorf("message %q; want the cause %q, that the scan did not read the file, and a reason of its own", message, cause)
	}
}

// A probe that fails without a verdict about the path is an error of the scan, not a path with nothing on it.
func TestAProbeThatReturnsNoVerdictIsAnErrorAndNotAnEmptyPath(t *testing.T) {
	probe := func(string, string) error { return errBoom }

	selection, err := canonicalOnDiskThrough(probe, "repo")

	if err == nil {
		t.Errorf("canonicalOnDiskThrough = %+v, nil; want the probe's failure returned", selection)
	}
}
