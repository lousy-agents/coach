package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy path that is the submodule itself, not a file inside it, has the same way out as one inside: the
// superproject records the submodule as one commit, so it can be neither restored nor committed here, and
// it cannot be a policy. It used to be called a git error when its checkout was removed, and "not a regular
// file" when the checkout was there.
func TestAPolicyPathThatIsASubmoduleItselfIsInTheSubmoduleWhetherOrNotItsCheckoutIsThere(t *testing.T) {
	rows := map[string]struct {
		checkoutPresent bool
		wantKind        ErrorKind
	}{
		"the checkout is removed": {false, KindMissing},
		"the checkout is there":   {true, KindInvalid},
	}
	for name, row := range rows {
		t.Run(name, func(t *testing.T) { requireTheSubmoduleItself(t, row.checkoutPresent, row.wantKind) })
	}
}

func requireTheSubmoduleItself(t *testing.T, checkoutPresent bool, wantKind ErrorKind) {
	t.Helper()
	repo := repositoryWithASubmodule(t)
	if checkoutPresent {
		if err := os.MkdirAll(filepath.Join(repo, "vendor", "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", "vendor/sub")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a refusal", status, err)
	}
	if status.PolicyCause != projectreadiness.CauseInSubmodule || status.Remedy != projectreadiness.RemedyNone || status.Kind != wantKind || status.GitFailure != nil {
		t.Errorf("kind %d, remedy %d, cause %q, git failure %v in %q; want kind %d, no remedy, cause in_submodule and no git failure", status.Kind, status.Remedy, status.PolicyCause, status.GitFailure, status.Message, wantKind)
	}
	if !strings.Contains(status.Message, `the submodule "vendor/sub"`) {
		t.Errorf("message %q does not name the submodule", status.Message)
	}
}

// Only the submodule's own path is the submodule: a directory below it, or a link inside its checkout, is
// refused for what it is on disk.
func TestAPathInsideASubmoduleCheckoutIsRefusedForWhatItIsOnDisk(t *testing.T) {
	rows := map[string]struct {
		arrange func(t *testing.T, repo string)
		path    string
		cause   string
	}{
		"a directory inside the checkout": {func(t *testing.T, repo string) {
			failOnError(t, os.MkdirAll(filepath.Join(repo, "vendor", "sub", "dir"), 0o755))
		}, "vendor/sub/dir", "not_a_regular_file"},
		"a link inside the checkout": {func(t *testing.T, repo string) {
			failOnError(t, os.MkdirAll(filepath.Join(repo, "vendor", "sub"), 0o755))
			failOnError(t, os.Symlink("target.json", filepath.Join(repo, "vendor", "sub", "link.json")))
		}, "vendor/sub/link.json", "symbolic_link"},
	}
	for name, row := range rows {
		t.Run(name, func(t *testing.T) { requireRefusedForWhatItIs(t, row.arrange, row.path, row.cause) })
	}
}

func requireRefusedForWhatItIs(t *testing.T, arrange func(t *testing.T, repo string), path, cause string) {
	t.Helper()
	repo := repositoryWithASubmodule(t)
	arrange(t, repo)

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", path)

	if err != nil || status == nil || string(status.PolicyCause) != cause {
		t.Errorf("ExplicitOnDiskStatus(%q) = %+v, %v; want a refusal with cause %q", path, status, err, cause)
	}
}

func failOnError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
