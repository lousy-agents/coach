package projectconfig

import (
	"errors"
	"os/exec"
	"testing"
)

func notARepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Run() == nil {
		t.Fatalf("%s is inside a git repository, so git can find a root there; point TMPDIR at a directory outside any repository", dir)
	}
	return dir
}

func TestWhenGitCannotFindTheRepositoryTheExplicitPolicyCheckFailsInsteadOfPassing(t *testing.T) {
	cases := []struct {
		name string
		call func(dir string) error
	}{
		{"ExplicitOnDiskStatus", func(dir string) error {
			status, err := ExplicitOnDiskStatus(runProjectConfigGit, dir, "HEAD", "p.json")
			if status != nil {
				t.Errorf("ExplicitOnDiskStatus returned the verdict %+v; an operational failure is never a verdict about the policy", status)
			}
			return err
		}},
		{"requireOnDisk", func(dir string) error { return requireOnDisk(runProjectConfigGit, dir, "p.json") }},
		{"LoadExplicit", func(dir string) error {
			config, err := LoadExplicit(dir, "HEAD", "p.json")
			if config != nil {
				t.Errorf("LoadExplicit returned a config %s although the working tree was never checked", config)
			}
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertOperationalFailure(t, c.call(notARepository(t)))
		})
	}
}

func assertOperationalFailure(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("got no error; the policy would be read from the committed copy without ever checking the working tree")
	}
	var verdict *ConfigError
	if errors.As(err, &verdict) {
		t.Errorf("error %q is a *ConfigError, a verdict about the policy; it must be an operational failure", err)
	}
}
