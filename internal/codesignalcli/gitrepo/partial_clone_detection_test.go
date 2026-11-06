package gitrepo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestIsPartialCloneReadsEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		want       bool
	}{
		{"a promisor remote", fakeRepository{promisors: promisorOrigin}, true},
		{"a promisor remote beside one that is not", fakeRepository{promisors: "remote.up.promisor false\nremote.origin.promisor true\n"}, true},
		{"a promisor remote set to false", fakeRepository{promisors: "remote.origin.promisor false\n"}, false},
		{"the extension older git wrote", fakeRepository{extension: "origin\n"}, true},
		{"no marker at all", fakeRepository{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPartialCloneRead(t, c.repository, c.want)
		})
	}
}

func assertPartialCloneRead(t *testing.T, repository fakeRepository, want bool) {
	t.Helper()
	if got, err := IsPartialClone(repository.run, "repo"); err != nil || got != want {
		t.Errorf("IsPartialClone = %v, %v; want %v and no error", got, err, want)
	}
}

// IsPartialClone reads a promisor setting the way git does, so it asks git to read it as a boolean. A
// key with no value at all is true to git, and so is any non-zero number; read as text, neither is.
func TestIsPartialCloneAsksGitToReadThePromisorSettingAsABoolean(t *testing.T) {
	var asked []string
	run := func(_ string, args ...string) ([]byte, error) {
		asked = append(asked, strings.Join(args, " "))
		return nil, exitStatusOne
	}

	_, _ = IsPartialClone(run, "repo")

	if !slices.ContainsFunc(asked, func(command string) bool {
		return strings.Contains(command, "--type=bool") && strings.Contains(command, "promisor")
	}) {
		t.Errorf("asked %q; want the promisor setting read with --type=bool", asked)
	}
}

func TestIsPartialCloneReadsThePromisorSettingsGitTakesForTrue(t *testing.T) {
	cases := []struct {
		name    string
		setting string
		want    bool
	}{
		{"no value at all", "\tpromisor\n", true},
		{"true", "\tpromisor = true\n", true},
		{"upper case", "\tpromisor = TRUE\n", true},
		{"yes", "\tpromisor = yes\n", true},
		{"on", "\tpromisor = on\n", true},
		{"one", "\tpromisor = 1\n", true},
		{"a number other than zero", "\tpromisor = 2\n", true},
		{"false", "\tpromisor = false\n", false},
		{"no", "\tpromisor = no\n", false},
		{"off", "\tpromisor = off\n", false},
		{"zero", "\tpromisor = 0\n", false},
		{"an empty value", "\tpromisor =\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPromisorSettingRead(t, c.setting, c.want)
		})
	}
}

func assertPromisorSettingRead(t *testing.T, setting string, want bool) {
	t.Helper()
	repository := gitfixture.Init(t)
	appendToGitConfig(t, repository, "[remote \"origin\"]\n"+setting)

	partial, err := IsPartialClone(RunBytes, repository)
	remote, named := PromisorRemote(RunBytes, repository)

	if err != nil || partial != want || named != want || (want && remote != "origin") {
		t.Errorf("IsPartialClone = %v, %v and PromisorRemote = %q, %v; want %v naming origin", partial, err, remote, named, want)
	}
}

// extensions.partialclone is what older git wrote to name the promisor remote; it holds the remote's name
// and not a boolean, and an empty one names nothing.
func TestIsPartialCloneReadsTheExtensionOlderGitWroteFromARealConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		want   bool
	}{
		{"a named remote", "upstream", true},
		{"an empty name", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertExtensionRead(t, c.remote, c.want)
		})
	}
}

func assertExtensionRead(t *testing.T, remote string, want bool) {
	t.Helper()
	repository := gitfixture.Init(t)
	gitOutput(t, repository, "config", "extensions.partialclone", remote)

	partial, err := IsPartialClone(RunBytes, repository)
	named, found := PromisorRemote(RunBytes, repository)

	if err != nil || partial != want || found != want || (want && named != remote) {
		t.Errorf("IsPartialClone = %v, %v and PromisorRemote = %q, %v; want %v naming %q", partial, err, named, found, want, remote)
	}
}

func TestAPromisorRemoteSettingIsReadBesideTheExtensionAndNeitherHidesTheOther(t *testing.T) {
	repository := gitfixture.Init(t)
	gitOutput(t, repository, "config", "remote.up.promisor", "false")
	gitOutput(t, repository, "config", "extensions.partialclone", "origin")

	named, found := PromisorRemote(RunBytes, repository)

	if !found || named != "origin" {
		t.Errorf("PromisorRemote = %q, %v; want origin from the extension when the only remote setting is false", named, found)
	}
}

func TestIsPartialCloneRefusesAPromisorSettingGitCannotReadAsABoolean(t *testing.T) {
	repository := gitfixture.Init(t)
	appendToGitConfig(t, repository, "[remote \"origin\"]\n\tpromisor = maybe\n")

	partial, err := IsPartialClone(RunBytes, repository)

	if err == nil || partial {
		t.Errorf("IsPartialClone = %v, %v; want an error and no answer for a setting git itself cannot read", partial, err)
	}
}

func TestIsPartialCloneReadsARemoteNamedWithASpace(t *testing.T) {
	repository := gitfixture.Init(t)
	appendToGitConfig(t, repository, "[remote \"my remote\"]\n\tpromisor = true\n")

	remote, named := PromisorRemote(RunBytes, repository)

	if !named || remote != "my remote" {
		t.Errorf("PromisorRemote = %q, %v; want the remote's whole name", remote, named)
	}
}

func appendToGitConfig(t *testing.T, repository, text string) {
	t.Helper()
	config, err := os.OpenFile(filepath.Join(repository, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open the configuration: %v", err)
	}
	defer func() { _ = config.Close() }()
	if _, err := config.WriteString(text); err != nil {
		t.Fatalf("write the configuration: %v", err)
	}
}

func TestIsPartialCloneSeesTheConfigurationGitWritesForAFilteredClone(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	ordinary := gitfixture.Init(t)

	if partial, err := IsPartialClone(RunBytes, clone); err != nil || !partial {
		t.Errorf("IsPartialClone(%s) = %v, %v; want true for a clone made with --filter", clone, partial, err)
	}
	if partial, err := IsPartialClone(RunBytes, ordinary); err != nil || partial {
		t.Errorf("IsPartialClone(%s) = %v, %v; want false for an ordinary repository", ordinary, partial, err)
	}
}

// A question git cannot answer is not answered "no": git's exit 1 is its way of saying nothing matched, and
// anything else (a damaged config, a timeout, a bound) says Coach does not know.
func TestIsPartialCloneDoesNotTakeAFailureOfGitForNo(t *testing.T) {
	cases := map[string]error{
		"a damaged configuration":        errors.New("exit status 128: fatal: bad config line 1 in file .git/config"),
		"a fatal exit that said nothing": exitStatus128,
		"a read past the time bound":     &BoundError{Kind: BoundTimeout, message: "git execution timed out"},
		"a read past the stderr bound":   &BoundError{Kind: BoundStderr, message: "git stderr exceeded the budget"},
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) { requireUnanswered(t, failure) })
	}
}

func requireUnanswered(t *testing.T, failure error) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return nil, failure }
	if partial, err := IsPartialClone(run, "repo"); err == nil || partial {
		t.Errorf("IsPartialClone = %v, %v; want an error and no answer", partial, err)
	}
}

func TestRefuseUnsafeLazyFetchRefusesARepositoryItCannotTellIsAPartialClone(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 128: fatal: bad config line 1 in file .git/config")
	}

	err := RefuseUnsafeLazyFetch(run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "could not tell whether this repository is a partial clone") || refusal.Cause == nil {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want a refusal that says it could not tell whether the repository is a partial clone, and keeps git's error", err)
	}
}

// A git that is not installed is said so in the words every other read of it uses, not as a question about
// partial clones.
func TestRefuseUnsafeLazyFetchSaysWhenGitIsNotInstalled(t *testing.T) {
	missing := &exec.Error{Name: "git", Err: exec.ErrNotFound}
	run := func(string, ...string) ([]byte, error) { return nil, missing }

	err := RefuseUnsafeLazyFetch(run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "git executable not found") {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want the message every read uses when git is not installed", err)
	}
}
