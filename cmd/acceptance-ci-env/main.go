// Command acceptance-ci-env runs Go tests under the conditions CI runs them
// in: a non-root user, an empty HOME, no user-level git configuration, CI's
// PATH, and CI's pinned git. It is `mise run acceptance-ci-env`. See
// docs/development/validation-and-ci.md.
package main

import (
	"fmt"
	"io"
	"os"
)

const prefix = "acceptance-ci-env: "

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(goTestArgs []string, stdout, stderr io.Writer) int {
	git, err := checkGit(os.Getenv("COACH_CI_GIT"), os.Getenv("COACH_CI_GIT_VERSION"), os.Getenv("COACH_CI_GIT_ALLOW_MISMATCH") == "1")
	if err != nil {
		fmt.Fprintln(stderr, prefix+err.Error())
		return 2
	}
	fmt.Fprintln(stdout, prefix+git.summary())
	if git.warning != "" {
		fmt.Fprintln(stdout, prefix+git.warning)
	}
	withJS, jsNote, err := jsPlan(goTestArgs)
	if err != nil {
		fmt.Fprintln(stderr, prefix+err.Error())
		return 2
	}
	if jsNote != "" {
		fmt.Fprintln(stdout, prefix+jsNote)
	}
	space, err := prepareWorkspace(git.path, withJS)
	if err != nil {
		fmt.Fprintln(stderr, prefix+err.Error())
		return 2
	}
	defer space.cleanup()
	if space.jsNote != "" {
		fmt.Fprintln(stdout, prefix+space.jsNote)
	}
	if len(goTestArgs) == 0 {
		goTestArgs = []string{"./..."}
	}
	fmt.Fprintf(stdout, "%srunning go test %v as uid %d, with an empty HOME (%s) and no user-level git config\n", prefix, goTestArgs, space.uid, space.home)
	return space.goTest(goTestArgs, stdout, stderr)
}
