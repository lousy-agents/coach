package evalhistory

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// git runs git offline and isolated: no network protocol, no hook, and no
// global, system or inherited GIT_* configuration.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "protocol.allow=never", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = isolatedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func isolatedEnv() []string {
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}
