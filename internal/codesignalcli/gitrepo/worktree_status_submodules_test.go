package gitrepo

import (
	"slices"
	"strings"
	"testing"
)

// statusRecorder answers the version and the status of a repository whose git prints version, and keeps the
// arguments of the status it was asked for.
type statusRecorder struct {
	version      string
	versionErr   error
	statusAsked  []string
	statusOutput string
}

func (r *statusRecorder) run(_ string, args ...string) ([]byte, error) {
	if args[0] == "version" {
		return []byte(r.version), r.versionErr
	}
	r.statusAsked = args
	return []byte(r.statusOutput), nil
}

func statusAskedOn(t *testing.T, recorder *statusRecorder) []string {
	t.Helper()
	original := runWorktreeStatusGit
	t.Cleanup(func() { runWorktreeStatusGit = original })
	runWorktreeStatusGit = recorder.run

	if _, err := WorktreeStatus("repo"); err != nil {
		t.Fatalf("WorktreeStatus = %v", err)
	}
	return recorder.statusAsked
}

// A git before 2.44 ignores GIT_NO_LAZY_FETCH, and `git status` goes into every populated submodule, where a
// partial clone that lacks objects is fetched into. So on such a git, and on one whose version cannot be read,
// the status does not go into submodules; on a git that can be told not to fetch, it is the plain status.
func TestTheStatusOfAGitBefore244DoesNotGoIntoSubmodules(t *testing.T) {
	for _, version := range []string{"git version 2.43.0", "git version 2.30.2", "git version 1.8.3"} {
		asked := statusAskedOn(t, &statusRecorder{version: version})

		if !slices.Contains(asked, "--ignore-submodules=all") {
			t.Errorf("%s asked %q; want the status not to go into submodules", version, asked)
		}
	}
}

func TestTheStatusOfAGitThatCanBeToldNotToFetchIsThePlainStatus(t *testing.T) {
	for _, version := range []string{"git version 2.44.0", "git version 2.54.0 (Apple Git-123)", "git version 3.0.0"} {
		asked := statusAskedOn(t, &statusRecorder{version: version})

		if got := strings.Join(asked, " "); got != "status --porcelain=v1 --untracked-files=all -z" {
			t.Errorf("%s asked %q; want the plain status", version, got)
		}
	}
}

func TestTheStatusOfAGitWhoseVersionCannotBeReadDoesNotGoIntoSubmodules(t *testing.T) {
	asked := statusAskedOn(t, &statusRecorder{versionErr: exitStatus128})

	if !slices.Contains(asked, "--ignore-submodules=all") {
		t.Errorf("asked %q; want the status not to go into submodules when the version is not known", asked)
	}
}

func TestTheStatusStillListsTheSuperprojectsPaths(t *testing.T) {
	recorder := &statusRecorder{version: "git version 2.43.0", statusOutput: " M a.go\x00?? b.go\x00"}
	original := runWorktreeStatusGit
	t.Cleanup(func() { runWorktreeStatusGit = original })
	runWorktreeStatusGit = recorder.run

	entries, err := WorktreeStatus("repo")

	if err != nil || len(entries) != 2 || entries[0].Path != "a.go" || entries[1].Path != "b.go" {
		t.Errorf("WorktreeStatus = %+v, %v; want the two paths", entries, err)
	}
}
