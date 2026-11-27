package validationtasks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const coachPackage = "github.com/lousy-agents/coach/cmd/coach"

// flagValues returns the value of every `-name=value` argument of command.
func flagValues(command, name string) []string {
	var values []string
	for _, word := range strings.Fields(command) {
		if value, found := strings.CutPrefix(word, "-"+name+"="); found {
			values = append(values, value)
		}
	}
	return values
}

// packageArguments returns the arguments of command that name packages, as go test reads them: those that start
// with ./, before the first flag that follows them.
func packageArguments(command string) []string {
	var packages []string
	for _, word := range strings.Fields(command) {
		if strings.HasPrefix(word, "./") {
			packages = append(packages, word)
		}
	}
	return packages
}

// jobRunCommands returns the run: value of each step of the job, as YAML decodes it.
func jobRunCommands(workflow, job string) []string {
	var runs []string
	steps, _ := workflowJob(workflow, job)["steps"].([]any)
	for _, step := range steps {
		fields, _ := step.(map[string]any)
		if run, ok := fields["run"].(string); ok {
			runs = append(runs, strings.TrimSpace(run))
		}
	}
	return runs
}

// goList returns the import paths the go tool names for the arguments, from the repository root.
func goList(arguments ...string) []string {
	GinkgoHelper()
	command := exec.Command("go", append([]string{"list"}, arguments...)...)
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	Expect(err).NotTo(HaveOccurred(), "go list %v", arguments)
	return strings.Fields(string(output))
}

// runSpecsEntryPoints names the Test function that holds each RunSpecs call of the Go test files in dir.
func runSpecsEntryPoints(dir string) []string {
	GinkgoHelper()
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	Expect(err).NotTo(HaveOccurred())
	entry := regexp.MustCompile(`(?s)func (Test\w+)\(t \*testing\.T\) \{[^}]*?\bRunSpecs\(`)
	var names []string
	for _, file := range files {
		raw, err := os.ReadFile(file)
		Expect(err).NotTo(HaveOccurred())
		var code []string
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				code = append(code, line)
			}
		}
		source := strings.Join(code, "\n")
		matches := entry.FindAllStringSubmatch(source, -1)
		for _, match := range matches {
			names = append(names, match[1])
		}
		if calls := strings.Count(source, "RunSpecs("); calls != len(matches) {
			names = append(names, filepath.Base(file)+" calls RunSpecs outside a Test function")
		}
	}
	return names
}

var _ = Describe("cmd/coach's acceptance specs split across two verify legs (#920, #605 Option B)", func() {
	var toml, yml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
		raw, err = os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
	})

	policyCommand := func(source string) string {
		commands := taskRunCommands(source, "test-policy")
		Expect(commands).To(HaveLen(1), "test-policy is one go test command")
		return commands[0]
	}
	restCommands := func(source string) []string {
		commands := taskRunCommands(source, "test-rest")
		Expect(commands).To(HaveLen(2), "test-rest is the other packages, then cmd/coach's remaining specs")
		return commands
	}

	Describe("the partition", func() {
		It("uses one pattern, once each, as the policy half's focus-file and the rest half's skip-file, so no spec runs in both halves or in neither (S1)", func() {
			focus := flagValues(policyCommand(toml), "ginkgo.focus-file")
			skip := flagValues(restCommands(toml)[1], "ginkgo.skip-file")

			Expect(focus).To(HaveLen(1))
			Expect(skip).To(HaveLen(1))
			Expect(focus[0]).NotTo(BeEmpty())
			Expect(skip[0]).To(Equal(focus[0]))
		})

		It("sees a pattern that differs between the halves (S1)", func() {
			mutant := strings.Replace(toml, "-ginkgo.skip-file=policy_", "-ginkgo.skip-file=policy", 1)
			Expect(mutant).NotTo(Equal(toml))

			Expect(flagValues(restCommands(mutant)[1], "ginkgo.skip-file")).NotTo(Equal(flagValues(policyCommand(mutant), "ginkgo.focus-file")))
		})

		It("runs the plain Test functions of cmd/coach in the rest half only: the policy half names the suite entry point with -run and the rest half names no -run (S2)", func() {
			Expect(strings.Fields(policyCommand(toml))).To(ContainElements("-run", "TestCoachAcceptance"))
			Expect(strings.Fields(restCommands(toml)[1])).NotTo(ContainElement("-run"))
			Expect(strings.Fields(restCommands(toml)[0])).NotTo(ContainElement("-run"))
		})

		It("has one suite entry point in cmd/coach, and it is the one the policy half's -run names, so no Ginkgo spec of the package is outside the policy half's reach (S7)", func() {
			entryPoints := runSpecsEntryPoints(filepath.Join("..", "..", "cmd", "coach"))

			Expect(entryPoints).To(Equal([]string{"TestCoachAcceptance"}))
			fields := strings.Fields(policyCommand(toml))
			Expect(fields[slices.Index(fields, "-run")+1]).To(Equal(entryPoints[0]))
		})

		It("counts a second suite entry point, wherever it is written (S7)", func() {
			dir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package x\n\nfunc TestA(t *testing.T) {\n\tRunSpecs(t, \"a\")\n}\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "b_test.go"), []byte("package x\n\nfunc TestB(t *testing.T) {\n\tRegisterFailHandler(Fail)\n\tRunSpecs(t, \"b\")\n}\n"), 0o644)).To(Succeed())

			Expect(runSpecsEntryPoints(dir)).To(Equal([]string{"TestA", "TestB"}))
		})
	})

	Describe("the packages", func() {
		It("names every package of the module but cmd/coach, each exactly once, in the rest half's first command (S3)", func() {
			arguments := packageArguments(restCommands(toml)[0])
			Expect(arguments).NotTo(BeEmpty())
			listed := map[string]int{}
			for _, argument := range arguments {
				for _, path := range goList(argument) {
					listed[path]++
				}
			}

			everything := goList("./...")
			Expect(everything).To(ContainElement(coachPackage))
			for _, path := range everything {
				if path == coachPackage {
					Expect(listed).NotTo(HaveKey(path), "cmd/coach runs in the rest half's second command, with the skip")
					continue
				}
				Expect(listed).To(HaveKeyWithValue(path, 1), "%s runs once in the rest half", path)
			}
			Expect(listed).To(HaveLen(len(everything) - 1))
		})

		It("runs cmd/coach once in the rest half, in the command that skips the policy specs, and once in the policy half (S3)", func() {
			Expect(packageArguments(restCommands(toml)[1])).To(Equal([]string{"./cmd/coach"}))
			Expect(packageArguments(policyCommand(toml))).To(Equal([]string{"./cmd/coach"}))
		})
	})

	Describe("the timeout", func() {
		DescribeTable("gives every go test command of the three test tasks the race detector and a 25 minute timeout (S4)",
			func(task string) {
				commands := taskRunCommands(toml, task)
				Expect(commands).NotTo(BeEmpty())
				for _, command := range commands {
					minutes, named := goTestTimeoutMinutes(command)
					Expect(named).To(BeTrue(), command)
					Expect(minutes).To(Equal(25), command)
					Expect(strings.Fields(command)).To(ContainElement("-race"), command)
				}
			},
			Entry("test, the whole run", "test"),
			Entry("test-rest", "test-rest"),
			Entry("test-policy", "test-policy"),
		)

		DescribeTable("leaves the job that runs the task five minutes more than go test's own timeout, so go test panics with its goroutine dump before Actions cancels the job (S4)",
			func(job, task string) {
				timeout, named := taskGoTestTimeout(toml, task)
				Expect(named).To(BeTrue())
				limit, named := jobTimeoutMinutes(yml, job)
				Expect(named).To(BeTrue())

				Expect(limit).To(BeNumerically(">=", timeout+5))
			},
			Entry("verify", "verify", "test-rest"),
			Entry("verify-policy", "verify-policy", "test-policy"),
		)
	})

	Describe("the jobs", func() {
		It("has verify run test-rest, and neither job run the whole test task, so no package runs twice (S5)", func() {
			verify, policy := jobRunCommands(yml, "verify"), jobRunCommands(yml, "verify-policy")

			Expect(verify).To(ContainElement("mise run test-rest"))
			Expect(verify).NotTo(ContainElement("mise run test"))
			Expect(policy).To(Equal([]string{"mise run test-policy"}))
		})

		It("gives verify-policy the setup verify has, so the two halves run in one environment (S5)", func() {
			verify, policy := workflowJob(yml, "verify"), workflowJob(yml, "verify-policy")
			Expect(policy).NotTo(BeNil())

			Expect(policy["runs-on"]).To(Equal(verify["runs-on"]))
			Expect(setupSteps(policy)).To(Equal(setupSteps(verify)))
			Expect(stepEnv(policy, "mise run test-policy")).To(Equal(stepEnv(verify, "mise run test-rest")))
		})

		It("lists verify-policy in status.needs, as every leaf must be (S6)", func() {
			Expect(needsList(jobBody(yml, "status"))).To(ContainElement("verify-policy"))
		})
	})
})

// setupSteps returns each uses: step of the job with its with: values, as text: the steps that make the machine
// (checkout, mise, the Go cache), and not the checks that run on it.
func setupSteps(job map[string]any) []string {
	var steps []string
	for _, raw := range job["steps"].([]any) {
		step, _ := raw.(map[string]any)
		if uses, ok := step["uses"].(string); ok {
			steps = append(steps, uses+" "+stringify(step["with"]))
		}
	}
	return steps
}

// stepEnv returns the env of the step of the job whose run is run.
func stepEnv(job map[string]any, run string) string {
	for _, raw := range job["steps"].([]any) {
		step, _ := raw.(map[string]any)
		if strings.TrimSpace(stringify(step["run"])) == run {
			return stringify(step["env"])
		}
	}
	return "no step runs " + run
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
