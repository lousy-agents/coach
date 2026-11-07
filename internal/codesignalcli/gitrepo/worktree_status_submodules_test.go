package gitrepo

import (
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

	if _, err := worktreeStatusBy(recorder.run, "repo"); err != nil {
		t.Fatalf("worktreeStatusBy = %v", err)
	}
	return recorder.statusAsked
}

const plainStatus = "status --porcelain=v1 --untracked-files=all -z"

// The arguments of the status are pinned for each kind of git, as the budgets are pinned for each call. A git before
// 2.44 ignores GIT_NO_LAZY_FETCH, and `git status` goes into every populated submodule, where a partial clone that
// lacks objects is fetched into; it also compares a staged deletion with a staged addition, which reads blobs. So on
// such a git, and on one whose version cannot be read, the status does not go into submodules; and from git 2.18,
// which has the flag, it does not look for renames on any git.
func TestTheArgumentsOfTheStatusForEachKindOfGit(t *testing.T) {
	for version, want := range map[string]string{
		"git version 2.43.0":                 plainStatus + " --no-renames --ignore-submodules=all",
		"git version 2.30.2":                 plainStatus + " --no-renames --ignore-submodules=all",
		"git version 2.18.0":                 plainStatus + " --no-renames --ignore-submodules=all",
		"git version 2.17.1":                 plainStatus + " --ignore-submodules=all",
		"git version 1.8.3":                  plainStatus + " --ignore-submodules=all",
		"git version 2.44.0":                 plainStatus + " --no-renames",
		"git version 2.54.0 (Apple Git-123)": plainStatus + " --no-renames",
		"git version 3.0.0":                  plainStatus + " --no-renames",
	} {
		assertStatusAskedAs(t, version, want)
	}
}

func assertStatusAskedAs(t *testing.T, version, want string) {
	t.Helper()
	t.Run(version, func(t *testing.T) {
		asked := statusAskedOn(t, &statusRecorder{version: version})

		if got := strings.Join(asked, " "); got != want {
			t.Errorf("%s asked %q; want %q", version, got, want)
		}
	})
}

func TestTheStatusOfAGitWhoseVersionCannotBeReadDoesNotGoIntoSubmodulesOrLookForRenames(t *testing.T) {
	asked := statusAskedOn(t, &statusRecorder{versionErr: exitStatus128})

	if got := strings.Join(asked, " "); got != plainStatus+" --ignore-submodules=all" {
		t.Errorf("asked %q; want the status that does not go into submodules, and no flag the version may not have", got)
	}
}

func TestTheStatusStillListsTheSuperprojectsPaths(t *testing.T) {
	recorder := &statusRecorder{version: "git version 2.43.0", statusOutput: " M a.go\x00?? b.go\x00"}

	entries, err := worktreeStatusBy(recorder.run, "repo")

	if err != nil || len(entries) != 2 || entries[0].Path != "a.go" || entries[1].Path != "b.go" {
		t.Errorf("worktreeStatusBy = %+v, %v; want the two paths", entries, err)
	}
}
