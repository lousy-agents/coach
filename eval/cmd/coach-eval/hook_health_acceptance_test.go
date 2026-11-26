package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	realCoachOnce sync.Once
	realCoach     string
)

// builtCoach is the coach built from this checkout, once per run: the hook runs the real review.
func builtCoach() string {
	realCoachOnce.Do(func() {
		realCoach = filepath.Join(buildDir, "coach")
		build := exec.Command("go", "build", "-o", realCoach, "./cmd/coach")
		build.Dir = repositoryRoot()
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	})
	return realCoach
}

func runHookHealth(stdin string, env []string, args ...string) generateOutcome {
	cmd := exec.Command(coachEvalBinary, append([]string{"hook-health"}, args...)...)
	cmd.Env = append(fixtureEnvironment(GinkgoT().TempDir()), env...)
	cmd.Stdin = strings.NewReader(stdin)
	return runGenerateCommand(cmd)
}

var (
	tuningMemo   = map[string]string{}
	tuningMemoMu sync.Mutex
)

// tuningLines is `coach-eval generate --tuning` output for a language, generated once per run: the twelve programs are
// validated natively, and every spec that needs them reads the same cache.
func tuningLines(language string) string {
	tuningMemoMu.Lock()
	defer tuningMemoMu.Unlock()
	if lines, ok := tuningMemo[language]; ok {
		return lines
	}
	args := []string{"--tuning"}
	if language != "" {
		args = append(args, "--language", language)
	}
	cache, err := os.MkdirTemp(buildDir, "tuning-cache")
	Expect(err).NotTo(HaveOccurred())
	outcome := runGenerate(cache, nil, args...)
	Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	tuningMemo[language] = outcome.stdout
	return outcome.stdout
}

// stubCoach writes a coach that prints the given report and exits with the given status.
func stubCoach(report string, status int) string {
	path := filepath.Join(GinkgoT().TempDir(), "coach")
	script := "#!/bin/sh\ncat <<'REPORT'\n" + report + "\nREPORT\nexit " + string(rune('0'+status)) + "\n"
	Expect(os.WriteFile(path, []byte(script), 0o755)).To(Succeed())
	return path
}

// programLine is a generate output line for a program tree that a spec built itself.
func programLine(path string) string {
	return `{"program_id":"hand-built","path":"` + path + `","cell":{"layers":3,"packages":2,"entry":"function","depth":2,"field":"scalar","direction":"read","trap":"layering"}}` + "\n"
}

// firstTuningProgramCopy is a writable copy of the first Go tuning program, to break on purpose.
func firstTuningProgramCopy() string {
	first := decodeAllGenerated(tuningLines(""))[0]
	dir := GinkgoT().TempDir()
	copyTree(readTree(first.Path), dir)
	return dir
}

var requiredTools = []string{"git", "jq", "sh"}

func allExcept(missing string) []string {
	var kept []string
	for _, tool := range requiredTools {
		if tool != missing {
			kept = append(kept, tool)
		}
	}
	return kept
}

// pathHolding is an environment whose PATH holds links to exactly the named tools.
func pathHolding(tools ...string) []string {
	dir := GinkgoT().TempDir()
	for _, tool := range tools {
		found, err := exec.LookPath(tool)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Symlink(found, filepath.Join(dir, tool))).To(Succeed())
	}
	return []string{"PATH=" + dir}
}

const cleanReport = `{"scope":{"policy":{"matches_head":true}},"signals":[],"diagnostics":[]}`

var _ = Describe("coach-eval hook-health (#789, design 7 and 10)", func() {
	It("finds the hook healthy on every Go tuning cell and the probe blocking on each", func() {
		outcome := runHookHealth(tuningLines(""), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		lines := strings.Split(strings.TrimSpace(outcome.stdout), "\n")
		Expect(lines).To(HaveLen(12))
		for _, line := range lines {
			Expect(line).To(HavePrefix("ok "))
		}
	})

	It("leaves the cached program as it was", func() {
		lines := tuningLines("")
		program := decodeAllGenerated(lines)[0]
		before := readTree(program.Path)

		outcome := runHookHealth(lines, nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(readTree(program.Path)).To(Equal(before))
	})

	It("exits 1 and names the stage when the hook never blocks, so the probe is not caught", func() {
		dir := firstTuningProgramCopy()
		Expect(os.WriteFile(filepath.Join(dir, "scripts", "coach-self-review.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())

		outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("hand-built"))
		Expect(outcome.stderr).To(ContainSubstring("probe"))
	})

	It("exits 1 and names the stage when the hook blocks the unmodified program", func() {
		dir := firstTuningProgramCopy()
		Expect(os.WriteFile(filepath.Join(dir, "scripts", "coach-self-review.sh"), []byte("#!/bin/sh\necho blocked >&2\nexit 2\n"), 0o755)).To(Succeed())

		outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("health"))
		Expect(outcome.stderr).To(ContainSubstring("blocked"))
	})

	It("exits 1 on a probe the review does not report, with health passing", func() {
		outcome := runHookHealth(programLine(firstTuningProgramCopy()), nil, "--coach", stubCoach(cleanReport, 0))

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("probe"))
		Expect(outcome.stderr).NotTo(ContainSubstring("health"))
	})

	DescribeTable("exits 1 when the hook does not block the probe the way it must",
		func(script string, mentions string) {
			dir := firstTuningProgramCopy()
			Expect(os.WriteFile(filepath.Join(dir, "scripts", "coach-self-review.sh"), []byte("#!/bin/sh\n"+script+"\n"), 0o755)).To(Succeed())

			outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

			Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("probe"))
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("it names the finding and the probe's path but exits 0",
			"echo architecture.layer_violation\nfind . -name zz_probe.go\nexit 0", "should block"),
		Entry("it blocks and names the path, but not the finding",
			"if find . -name zz_probe.go | grep -q .; then find . -name zz_probe.go >&2; exit 2; fi\nexit 0", "without naming"),
		Entry("it blocks and names the finding, but not the probe's path",
			"if find . -name zz_probe.go | grep -q .; then echo architecture.layer_violation >&2; exit 2; fi\nexit 0", "without naming"),
		Entry("it names the finding and the probe's path but exits 1",
			"if find . -name zz_probe.go | grep -q .; then echo architecture.layer_violation >&2; find . -name zz_probe.go >&2; exit 1; fi\nexit 0", "exited 1"),
		Entry("it names the finding and the probe's path but exits 3",
			"if find . -name zz_probe.go | grep -q .; then echo architecture.layer_violation >&2; find . -name zz_probe.go >&2; exit 3; fi\nexit 0", "exited 3"),
	)

	DescribeTable("exits 1 when the review is not clean on the unmodified program",
		func(report string, mentions string) {
			outcome := runHookHealth(programLine(firstTuningProgramCopy()), nil, "--coach", stubCoach(report, 0))

			Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("health"))
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("the policy on disk differs from HEAD", `{"scope":{"policy":{"matches_head":false}},"signals":[],"diagnostics":[]}`, "matches_head"),
		Entry("the report does not say whether the policy matches HEAD", `{"scope":{"policy":{}},"signals":[],"diagnostics":[]}`, "matches_head"),
		Entry("no policy was read", `{"scope":{"policy":null},"signals":[],"diagnostics":[]}`, "matches_head"),
		Entry("the worktree scan was incomplete", `{"scope":{"policy":{"matches_head":true}},"signals":[],"diagnostics":[{"kind":"worktree_scan_incomplete"}]}`, "worktree_scan_incomplete"),
	)

	DescribeTable("exits 2 for a tool or an input that is missing, before running anything",
		func(stdin func() string, env []string, args []string, mentions string) {
			outcome := runHookHealth(stdin(), env, args...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("no --coach", func() string { return tuningLines("") }, nil, []string{}, "--coach"),
		Entry("a coach that does not exist", func() string { return tuningLines("") }, nil, []string{"--coach", "/nonexistent/coach"}, "/nonexistent/coach"),
		Entry("no program on standard input", func() string { return "" }, nil, []string{"--coach", "/bin/true"}, "program"),
		Entry("a line that is not a program", func() string { return "not json\n" }, nil, []string{"--coach", "/bin/true"}, "program"),
		Entry("a program path that does not exist", func() string { return programLine("/nonexistent/program") }, nil, []string{"--coach", "/bin/true"}, "program"),
	)

	DescribeTable("exits 2 and names a required tool that PATH does not hold, whichever of the three it is",
		func(missing string) {
			outcome := runHookHealth(tuningLines(""), pathHolding(allExcept(missing)...), "--coach", "/bin/true")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring(missing + " is not on PATH"))
		},
		Entry("git", "git"),
		Entry("jq", "jq"),
		Entry("sh", "sh"),
	)

	It("exits 2, not 1, when a check cannot run at all, because that says nothing about the hook", func() {
		dir := firstTuningProgramCopy()
		Expect(os.Symlink("/nonexistent/target", filepath.Join(dir, "dangling"))).To(Succeed())

		outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("dangling"))
		Expect(outcome.stderr).NotTo(ContainSubstring("FAIL"))
	})

	It("exits 2 for a program whose forbidden edge names a layer the policy does not define", func() {
		dir := firstTuningProgramCopy()
		Expect(os.WriteFile(filepath.Join(dir, ".lousy-agents", "coach.json"),
			[]byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"api","prefixes":["internal/api"]}],"forbidden_imports":[{"from":"api","to":"nowhere"}]}`), 0o644)).To(Succeed())

		outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("does not define"))
	})

	It("exits 2 for a program whose policy names no forbidden edge to probe", func() {
		dir := firstTuningProgramCopy()
		Expect(os.WriteFile(filepath.Join(dir, ".lousy-agents", "coach.json"),
			[]byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"api","prefixes":["internal/api"]}]}`), 0o644)).To(Succeed())

		outcome := runHookHealth(programLine(dir), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("forbidden"))
	})

	It("finds the hook healthy on every TypeScript tuning cell and the probe blocking on each", Label(typeScriptLabel), func() {
		outcome := runHookHealth(tuningLines("typescript"), nil, "--coach", builtCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(strings.Split(strings.TrimSpace(outcome.stdout), "\n")).To(HaveLen(12))
	})
})
