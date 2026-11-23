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

// notAFileRow is a revision that commits the policy path as something else, how readiness names it when the policy at
// the revision is read, and what it says of a regular file on disk at the same path.
type notAFileRow struct {
	commit func(t *testing.T, repo string)
	cause  projectreadiness.PolicyCause
	named  string
	// warning is what the check says of a regular file on disk at the path, or gap is the gap it reports in its place.
	warning string
	gap     string
}

func notAFileRows() map[string]notAFileRow {
	return map[string]notAFileRow{
		"a symbolic link":   {func(t *testing.T, repo string) { gitfixture.CommitLink(t, repo, nonFilePolicy, "real.json") }, "committed_as_link", "symbolic link", projectreadiness.WarnPolicyDiffersFromHead, ""},
		"a directory":       {func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, nonFilePolicy+"/inner.json", "{}") }, "directory", "directory", projectreadiness.WarnPolicyNotCommitted, ""},
		"a submodule entry": {func(t *testing.T, repo string) { gitfixture.CommitGitlink(t, repo, nonFilePolicy) }, "in_submodule", "submodule", "", projectreadiness.GapPolicyMissing},
	}
}

// The policy the revision commits as a link, a directory or a submodule is not a policy to author over, restore or call
// missing: reading it at the revision says what the entry is and offers nothing.
func TestTheCommittedCopyOfAPolicyCommittedAsSomethingOtherThanAFileIsNamedForWhatItIs(t *testing.T) {
	for name, row := range notAFileRows() {
		t.Run(name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			gitfixture.CommitFile(t, repo, "a.txt", "x")
			row.commit(t, repo)

			got, roots, err := checkPolicy(readinessGit(), repo, "HEAD", nonFilePolicy)

			requireAVerdict(t, roots, err)
			requireAPolicyInvalidGap(t, got, row)
		})
	}
}

// A regular file on disk is the policy whatever the revision commits at its path, and the result says how the two
// differ. A submodule is the exception: the superproject can never commit a file there.
func TestARegularFileOnDiskWherePolicyIsCommittedAsSomethingOtherThanAFileIsThePolicy(t *testing.T) {
	for name, row := range notAFileRows() {
		t.Run(name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			gitfixture.CommitFile(t, repo, "a.txt", "x")
			row.commit(t, repo)
			writeRegularFile(t, filepath.Join(repo, nonFilePolicy))

			judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Named(nonFilePolicy))

			if err != nil {
				t.Fatalf("judgePolicy: %v", err)
			}
			if row.gap != "" {
				if judgment.check.Code != row.gap || judgment.check.PolicyCause != row.cause {
					t.Errorf("check = %+v; want %s with the cause %q", judgment.check, row.gap, row.cause)
				}
				return
			}
			if judgment.check.State != projectreadiness.Pass || len(judgment.warnings) != 1 || judgment.warnings[0].Code != row.warning {
				t.Errorf("check = %+v, warnings %+v; want a pass that says %q", judgment.check, judgment.warnings, row.warning)
			}
		})
	}
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
