package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

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

// A remote's name may hold a dot, and git writes the setting under it as it is. The fake evaluates the pattern
// the code passes, and this reads it from a real configuration, so narrowing the pattern to names without a
// dot fails both.
func TestAPromisorRemoteWhoseNameHoldsADotIsRecognised(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		setting string
		want    bool
	}{
		{"a dot in the name", "my.remote", "true", true},
		{"two dots in the name", "a.b.c", "true", true},
		{"a dot and a space", "my.remote 2", "true", true},
		{"a dot, set to false", "my.remote", "false", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repository := gitfixture.Init(t)
			appendToGitConfig(t, repository, "[remote \""+c.remote+"\"]\n\tpromisor = "+c.setting+"\n")

			partial, err := IsPartialClone(RunBytes, repository)
			named, found := PromisorRemote(RunBytes, repository)

			if err != nil || partial != c.want || found != c.want || (c.want && named != c.remote) {
				t.Errorf("IsPartialClone = %v, %v and PromisorRemote = %q, %v; want %v naming %q", partial, err, named, found, c.want, c.remote)
			}
		})
	}
}
