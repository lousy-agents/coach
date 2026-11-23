package acceptancecienv

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const repoRoot = "../.."

const probePackage = "./internal/acceptancecienv/testdata/probe"

type probeRecord struct {
	UID               int               `json:"uid"`
	Home              string            `json:"home"`
	HomeEntries       []string          `json:"home_entries"`
	HomeReadError     string            `json:"home_read_error"`
	TmpDir            string            `json:"tmpdir"`
	TmpDirEntries     []string          `json:"tmpdir_entries"`
	TmpDirReadError   string            `json:"tmpdir_read_error"`
	GlobalConfig      string            `json:"global_config"`
	GlobalConfigError bool              `json:"global_config_error"`
	GitVersion        string            `json:"git_version"`
	GoOnPath          bool              `json:"go_on_path"`
	ConfigFromEnv     []string          `json:"config_from_env"`
	ConfigScopes      []string          `json:"config_scopes"`
	ScrubbedSet       []string          `json:"scrubbed_set"`
	JSInstalled       map[string]string `json:"js_installed"`
}

type taskRun struct {
	stdout, stderr string
	exitCode       int
	probe          *probeRecord
}

var gitVersionPattern = regexp.MustCompile(`git version (\d+\.\d+\.\d+)`)

// goRunExit is the line `go run` prints for a program that exited non-zero, as
// it exits 1 itself whatever code the program ended with.
var goRunExit = regexp.MustCompile(`(?m)^exit status (\d+)$`)

// hostGitVersion is the version of the git this spec process finds on PATH.
func hostGitVersion() string {
	out, err := exec.Command("git", "version").Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	match := gitVersionPattern.FindStringSubmatch(string(out))
	ExpectWithOffset(1, match).NotTo(BeNil(), "git version printed %q", out)
	return match[1]
}

// runTask runs the acceptance-ci-env program from the repository root over the
// probe package, with the pin and any extra environment given, and returns what
// the probe recorded, if it ran.
func runTask(pin string, extraEnv ...string) taskRun {
	return runTaskWith(pin, []string{probePackage}, extraEnv...)
}

func runTaskWith(pin string, goTestArgs []string, extraEnv ...string) taskRun {
	return runTaskIn(repoRoot, pin, goTestArgs, extraEnv...)
}

// runTaskIn is runTaskWith from the repository at dir.
func runTaskIn(dir, pin string, goTestArgs []string, extraEnv ...string) taskRun {
	outDir, err := os.MkdirTemp("", "acceptance-ci-env-probe-*")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, outDir)
	ExpectWithOffset(1, os.Chmod(outDir, 0o777)).To(Succeed())
	probeOut := filepath.Join(outDir, "probe.json")

	command := exec.Command("go", append([]string{"run", "./cmd/acceptance-ci-env"}, goTestArgs...)...)
	command.Dir = dir
	command.Env = append(environWithout("COACH_CI_GIT", "COACH_CI_GIT_ALLOW_MISMATCH", "COACH_CI_GIT_VERSION"), "COACH_CI_GIT_VERSION="+pin, "COACH_CI_ENV_PROBE_OUT="+probeOut)
	command.Env = append(command.Env, extraEnv...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	run := taskRun{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		run.exitCode = exitErr.ExitCode()
		if match := goRunExit.FindStringSubmatch(run.stderr); match != nil {
			run.exitCode, _ = strconv.Atoi(match[1])
		}
	default:
		Fail("running the task: " + err.Error())
	}
	if data, readErr := os.ReadFile(probeOut); readErr == nil {
		run.probe = &probeRecord{}
		ExpectWithOffset(1, json.Unmarshal(data, run.probe)).To(Succeed(), "probe record: %s", data)
	}
	return run
}

// environWithout is this process's environment less the named variables, so a
// spec run inside the task does not inherit the outer run's git choices.
func environWithout(names ...string) []string {
	var kept []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		drop := false
		for _, unwanted := range names {
			drop = drop || name == unwanted
		}
		if !drop {
			kept = append(kept, entry)
		}
	}
	return kept
}

func (r taskRun) output() string { return r.stdout + r.stderr }

func readRepoFile(name string) string {
	data, err := os.ReadFile(filepath.Join(repoRoot, name))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return string(data)
}

func lines(text string) []string { return strings.Split(text, "\n") }

// expectOwnDirectory asserts that a directory the task gave the tests is its
// own and readable: not the caller's, and not one the probe could not read,
// which would look empty too. It returns the directory's entries.
func expectOwnDirectory(variable, dir string, entries []string, readErr string) []string {
	ExpectWithOffset(1, dir).NotTo(BeEmpty(), "%s must be set", variable)
	ExpectWithOffset(1, dir).NotTo(Equal(os.Getenv(variable)), "%s must not be the caller's", variable)
	ExpectWithOffset(1, readErr).To(BeEmpty(), "%s must be readable by the tests", variable)
	return entries
}

// repositoryCopy copies .git and the files git does not ignore into a new
// directory, so a spec can add or leave out an ignored file such as the
// installed js/semantics without touching the caller's tree.
func repositoryCopy() string {
	target, err := os.MkdirTemp("", "acceptance-ci-env-repo-*")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, target)
	listing := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	listing.Dir = repoRoot
	names, err := listing.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	copying := exec.Command("cp", "-a", "--parents", "--no-dereference", "--target-directory="+target, ".git")
	copying.Dir = repoRoot
	out, err := copying.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "%s", out)
	archive := exec.Command("sh", "-c", `tar --null --files-from=- --ignore-failed-read -cf - | tar -xf - -C "$1"`, "sh", target)
	archive.Dir = repoRoot
	archive.Stdin = bytes.NewReader(names)
	out, err = archive.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "%s", out)
	return target
}
