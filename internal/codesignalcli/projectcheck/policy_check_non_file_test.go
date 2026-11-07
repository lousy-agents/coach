package projectcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const nonFilePolicy = "cfg/p.json"

// notAFileRow is a revision that commits the policy path as something else, and how readiness names it.
type notAFileRow struct {
	commit func(t *testing.T, repo string)
	cause  projectreadiness.PolicyCause
	named  string
}

func notAFileRows() map[string]notAFileRow {
	return map[string]notAFileRow{
		"a symbolic link":   {func(t *testing.T, repo string) { gitfixture.CommitLink(t, repo, nonFilePolicy, "real.json") }, "committed_as_link", "symbolic link"},
		"a directory":       {func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, nonFilePolicy+"/inner.json", "{}") }, "directory", "directory"},
		"a submodule entry": {func(t *testing.T, repo string) { gitfixture.CommitGitlink(t, repo, nonFilePolicy) }, "in_submodule", "submodule"},
	}
}

// A policy the revision commits as a link, a directory or a submodule is not a policy to author over,
// restore or call missing: readiness says what the entry is and offers nothing, for a discovered policy
// and for one the user named whose working tree holds a regular file at the path.
func TestReadinessNamesAPolicyCommittedAsSomethingOtherThanAFile(t *testing.T) {
	for name, row := range notAFileRows() {
		t.Run(name+", discovered", func(t *testing.T) { requireNamedAs(t, row, false) })
		t.Run(name+", named, with a file on disk", func(t *testing.T) { requireNamedAs(t, row, true) })
	}
}

func requireNamedAs(t *testing.T, row notAFileRow, named bool) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	row.commit(t, repo)
	check := func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
		return checkPolicy(readinessGit(), dir, revision, policyPath)
	}
	if named {
		writeRegularFile(t, filepath.Join(repo, nonFilePolicy))
		check = policyCheckerBy(readinessGit(), policyref.Named(nonFilePolicy))
	}

	got, roots, err := check(repo, "HEAD", nonFilePolicy)

	requireAVerdict(t, roots, err)
	requireAPolicyInvalidGap(t, got, row)
}

func requireAVerdict(t *testing.T, roots []string, err error) {
	t.Helper()
	if err != nil || roots != nil {
		t.Fatalf("check = roots %v, error %v; want a verdict", roots, err)
	}
}

func requireAPolicyInvalidGap(t *testing.T, got projectreadiness.Check, row notAFileRow) {
	t.Helper()
	want := projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid, Remedy: projectreadiness.RemedyNone}
	want.PolicyCause, want.CauseText = row.cause, got.CauseText
	if got.State != want.State || got.Code != want.Code || got.Remedy != want.Remedy || got.PolicyCause != want.PolicyCause {
		t.Errorf("check = %+v; want a policy_invalid gap with no remedy and cause %q", got, row.cause)
	}
	if !strings.Contains(got.CauseText, row.named) {
		t.Errorf("cause text %q does not call the entry a %s", got.CauseText, row.named)
	}
}

// writeRegularFile replaces whatever is at path with a regular file, as a user does who has the
// policy open in an editor while the revision commits something else there.
func writeRegularFile(t *testing.T, path string) {
	t.Helper()
	failOnError(t, os.RemoveAll(path))
	failOnError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	failOnError(t, os.WriteFile(path, []byte(`{"schema_version":"1","roots":["."]}`), 0o644))
}

func failOnError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
