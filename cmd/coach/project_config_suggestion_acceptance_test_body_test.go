package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectConfigSuggestionAcceptanceTest_removesThePartiallyWrittenTargetAndReportsProjec_360() {
	if runtime.GOOS == "windows" {
		Skip("ulimit -f is not available on windows")
	}
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/midwrite\n\ngo 1.25\n")

	// ulimit -f 0 lets the create-only O_EXCL open succeed but makes
	// the very next Write fail with EFBIG, exercising the same
	// write-failure branch as ENOSPC/EIO without needing root or a
	// size-limited filesystem. Stderr must be a bytes.Buffer, not an
	// *os.File: an *os.File stderr is also subject to the file-size
	// limit and the run degrades to exit 1 with an empty envelope,
	// which would be a false green for this branch.
	command := exec.Command("sh", "-c", "ulimit -f 0; exec "+commandPath+" codesignal --baseline --suggest-project-config --output out.json")
	command.Dir = repo
	var stderr bytes.Buffer
	command.Stderr = &stderr

	err := command.Run()
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, stderr.String())
	Expect(exitErr.ExitCode()).To(Equal(2))
	Expect(stderr.String()).To(ContainSubstring("project_config_suggestion_output_invalid"))
	Expect(stderr.String()).NotTo(ContainSubstring(repo), "the envelope must not leak the absolute repository path")

	_, statErr := os.Stat(filepath.Join(repo, "out.json"))
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "the partially-written target must be removed on write failure")
}

func body_projectConfigSuggestionAcceptanceTest_exits2WithProjectConfigSuggestionOutputInvalidIn_436() {
	if os.Geteuid() == 0 {
		Skip("cannot exercise a permission-denied write while running as root")
	}
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/readonlydir\n\ngo 1.25\n")
	readonlyDir := filepath.Join(repo, "readonly")
	Expect(os.MkdirAll(readonlyDir, 0o755)).To(Succeed())
	Expect(os.Chmod(readonlyDir, 0o555)).To(Succeed())
	DeferCleanup(func() { os.Chmod(readonlyDir, 0o755) })

	stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "readonly/out.json")
	Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
	Expect(stdout).To(BeEmpty())
	Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
	Expect(string(stderr)).NotTo(ContainSubstring(repo), "the envelope must not leak the absolute repository path")
}

func body_projectConfigSuggestionAcceptanceTest_rejectsAGitPathComponentRegardlessOfCaseWithoutR_591() {
	// A single-segment target (not ".GIT/project.json") deliberately
	// avoids checkOutputParents' parent-existence Lstat: on this
	// case-sensitive Linux filesystem, a nested ".GIT/..." target
	// would already fail there (only ".git" exists on disk) even
	// without the case-insensitive shape check this test exists to
	// prove, which would be a false green. A bare "--output .GIT"
	// has zero parent segments to Lstat, so the only thing that can
	// reject it is validateSuggestOutputPathShape's segment check.
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/gitcomponentcase\n\ngo 1.25\n")

	stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", ".GIT")
	Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
	Expect(stdout).To(BeEmpty())
	Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))

	// On case-insensitive filesystems (APFS/HFS+), Lstat(".GIT") resolves
	// to the existing .git directory — prove no regular-file candidate
	// was written, not that the path is absent.
	info, statErr := os.Lstat(filepath.Join(repo, ".GIT"))
	if statErr == nil {
		Expect(info.Mode().IsRegular()).To(BeFalse(), "must not write a regular-file candidate at the rejected .GIT path")
	} else {
		Expect(os.IsNotExist(statErr)).To(BeTrue(), "unexpected Lstat error for rejected .GIT path: %v", statErr)
	}
}

func body_projectConfigSuggestionAcceptanceTest_uses2SpaceIndentationForTheStdoutCandidateAndCom_636() {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", "module example.com/shape\n\ngo 1.25\n")

	stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	Expect(string(stdout)).To(Equal("{\n  \"schema_version\": \"1\",\n  \"roots\": [\n    \".\"\n  ]\n}\n"))

	text := string(stderr)
	Expect(strings.Count(text, "\n")).To(Equal(1), "expected exactly one newline, at the very end, for NDJSON: got %q", text)
	Expect(strings.HasSuffix(text, "\n")).To(BeTrue())
	Expect(text).NotTo(ContainSubstring("\n  \""), "expected compact single-line JSON, not pretty-printed multi-line JSON")
	order := []string{`"diagnostic_version"`, `"kind"`, `"revision"`, `"heuristic_version"`, `"roots_considered"`, `"coverage"`, `"diagnostics"`}
	lastIndex := -1
	for _, key := range order {
		idx := strings.Index(text, key)
		Expect(idx).To(BeNumerically(">", lastIndex), "expected %s to appear after the previous key in %s", key, text)
		lastIndex = idx
	}
}
