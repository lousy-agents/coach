package evalcensus

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// isolatedEnv is the environment for every git and Coach process the census
// starts: no inherited GIT_* setting, and no global or system git
// configuration, so no filter, hook or helper the host or the repository
// defines can run.
func isolatedEnv() []string {
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}

// git runs git with hooks disabled in the isolated environment.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = isolatedEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(output)), nil
}

// CheckoutRepository clones repo into dir and checks out its SHA. The clone
// copies objects without running the source's upload-pack, and its fresh
// configuration defines no filter, so the source repository's own
// configuration and hooks never run.
func CheckoutRepository(repo Repository, dir string) error {
	if _, err := git("", "clone", "--quiet", "--no-checkout", "--no-hardlinks", repo.Path, dir); err != nil {
		return fmt.Errorf("repository %q: %w", repo.ID, err)
	}
	if _, err := git(dir, "cat-file", "-e", repo.SHA+"^{commit}"); err != nil {
		return fmt.Errorf("repository %q: SHA %s is not reachable in %s", repo.ID, repo.SHA, repo.Path)
	}
	if _, err := git(dir, "checkout", "--quiet", "--detach", repo.SHA); err != nil {
		return fmt.Errorf("repository %q: %w", repo.ID, err)
	}
	return nil
}
