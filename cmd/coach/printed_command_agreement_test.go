package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/invocation"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

var pathsPrintedAlike = []struct {
	name string
	path string
}{
	{"a plain path", "policies/p.json"},
	{"a no-break space", "my\u00a0policy.json"},
	{"an ideographic space", "my\u3000policy.json"},
	{"a private-use character", "my\ue000policy.json"},
	{"an escape character", "bad\x1bname.json"},
	{"a zero-width space", "my\u200bpolicy.json"},
	{"a line separator", "my\u2028policy.json"},
	{"a stray C1 byte", "my\x9bpolicy.json"},
	{"a newline", "my\npolicy.json"},
}

func TestRemediationAndSeeAllHintAgreeOnWhatCanBePrinted(t *testing.T) {
	for _, c := range pathsPrintedAlike {
		t.Run(c.name, func(t *testing.T) {
			assertSurfacesAgreeOnPrintability(t, c.path)
		})
	}
}

func assertSurfacesAgreeOnPrintability(t *testing.T, path string) {
	t.Helper()
	remediation := invocation.Command{Mode: invocation.CheckProject, Policy: policyref.Named(path)}.Phrase("run")
	seeAll := seeAllCommand([]string{"--project-config", path, "--top", "1"})

	remediationShows := !strings.Contains(remediation, "not printed here")
	seeAllShows := seeAll != ""
	if remediationShows != seeAllShows {
		t.Errorf("path %q: remediation shows it = %v (%q), the see-all hint shows it = %v (%q)", path, remediationShows, remediation, seeAllShows, seeAll)
	}
}

func TestRemediationAndSeeAllHintQuoteAPathTheSameWay(t *testing.T) {
	paths := []string{"policies/p.json", "a@b.json", "a=b.json", "=lead.json", "x,y.json", "50%.json", "a+b.json", "my policy.json", "it's.json", "~/p.json", "a^b.json", "$(touch PWNED).json"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			assertSurfacesQuoteAlike(t, path)
		})
	}
}

func assertSurfacesQuoteAlike(t *testing.T, path string) {
	t.Helper()
	remediation := invocation.Command{Mode: invocation.CheckProject, Policy: policyref.Named(path)}.Phrase("")
	_, inRemediation, _ := strings.Cut(remediation, " --project-config ")
	seeAll := seeAllCommand([]string{"--project-config", path, "--top", "1"})
	_, inSeeAll, _ := strings.Cut(seeAll, " --project-config ")

	if inRemediation != inSeeAll {
		t.Errorf("path %q is written as %q in the remediation and %q in the see-all hint", path, inRemediation, inSeeAll)
	}
}
