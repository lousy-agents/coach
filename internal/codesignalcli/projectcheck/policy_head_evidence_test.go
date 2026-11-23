package projectcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Each row differs from the row that is quiet by the one clause the warning decides on, so a clause that stopped
// deciding would change a row.
func TestTheWarningForAPolicyOnDiskNamesHowItRelatesToTheAnalyzedRevision(t *testing.T) {
	quiet := headEvidence{copy: headHoldsAFile, sameBytes: true}
	cases := []struct {
		name     string
		evidence headEvidence
		want     string
	}{
		{"the file is the committed copy", quiet, ""},
		{"the committed copy differs", headEvidence{copy: headHoldsAFile}, projectreadiness.WarnPolicyDiffersFromHead},
		{"the revision holds nothing at the path", headEvidence{copy: headHoldsNoFile}, projectreadiness.WarnPolicyNotCommitted},
		{"the revision holds a link", headEvidence{copy: headHoldsALink, sameBytes: true}, projectreadiness.WarnPolicyDiffersFromHead},
		{"the index holds a conflict over the same bytes", headEvidence{copy: headHoldsAFile, sameBytes: true, conflicted: true}, projectreadiness.WarnPolicyDiffersFromHead},
		{"the index holds a conflict and the revision holds nothing", headEvidence{copy: headHoldsNoFile, conflicted: true}, projectreadiness.WarnPolicyDiffersFromHead},
		{"a question went unanswered over the same bytes", headEvidence{copy: headHoldsAFile, sameBytes: true, unanswered: true}, projectreadiness.WarnPolicyHeadUnreadable},
		{"a question went unanswered and the revision holds nothing", headEvidence{copy: headHoldsNoFile, unanswered: true}, projectreadiness.WarnPolicyHeadUnreadable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.evidence.warningCode(); got != c.want {
				t.Errorf("warningCode(%+v) = %q, want %q", c.evidence, got, c.want)
			}
		})
	}
}

func selectionOf(t *testing.T, repo, path string) projectconfig.DiskSelection {
	t.Helper()
	selection, err := projectconfig.SelectOnDisk(repo, path)
	if err != nil || selection.Path != path {
		t.Fatalf("SelectOnDisk(%q) = %+v, %v; want the file selected", path, selection, err)
	}
	return selection
}

func TestTheEvidenceIsReadFromTheRevisionAndTheIndex(t *testing.T) {
	const path = "cfg/p.json"
	cases := []struct {
		name    string
		arrange func(t *testing.T, repo string)
		want    string
	}{
		{"a file nothing commits", func(t *testing.T, repo string) { writeWorktreeFile(t, repo, path, validPolicy) }, projectreadiness.WarnPolicyNotCommitted},
		{"the committed file", func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, path, validPolicy) }, ""},
		{"an edit of the committed file", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, path, `{"schema_version":"1","roots":["src"]}`)
			writeWorktreeFile(t, repo, path, validPolicy)
		}, projectreadiness.WarnPolicyDiffersFromHead},
		{"a link committed at the path", func(t *testing.T, repo string) {
			gitfixture.CommitLink(t, repo, path, "real.json")
			if err := os.RemoveAll(filepath.Join(repo, path)); err != nil {
				t.Fatal(err)
			}
			writeWorktreeFile(t, repo, path, validPolicy)
		}, projectreadiness.WarnPolicyDiffersFromHead},
		{"the index held by another git process", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, path, validPolicy)
			writeWorktreeFile(t, repo, ".git/index.lock", "")
		}, projectreadiness.WarnPolicyHeadUnreadable},
		{"a committed blob git cannot read", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, path, validPolicy)
			removeCommittedBlob(t, repo, path)
		}, projectreadiness.WarnPolicyHeadUnreadable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			gitfixture.CommitFile(t, repo, "a.txt", "x")
			c.arrange(t, repo)

			evidence := readHeadEvidence(readinessGit(), repo, "HEAD", selectionOf(t, repo, path))

			if got := evidence.warningCode(); got != c.want {
				t.Errorf("warning = %q for %+v, want %q", got, evidence, c.want)
			}
		})
	}
}
