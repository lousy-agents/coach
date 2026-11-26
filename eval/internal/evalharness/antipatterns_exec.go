package evalharness

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// isolatedGitEnv is the host environment without any GIT_* setting and with no global or system git configuration: the
// variants are committed and scanned in repositories of their own, so a GIT_DIR from a calling hook or a host hooks
// path shall not reach them.
func isolatedGitEnv() []string {
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}

// runIn runs a program in dir with extra environment entries and standard input, and returns its combined output.
func runIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(isolatedGitEnv(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}
