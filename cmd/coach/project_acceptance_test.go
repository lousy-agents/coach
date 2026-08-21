package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runInProcess invokes run() directly (in-process, no subprocess) so tests
// can override loadProjectConfig/resolveProjectBackend and observe
// runCodesignal's exit-code classification for error types codesignalcli's
// real implementations never produce.
func runInProcess(args ...string) (stdout, stderr []byte, exitCode int) {
	outFile, err := os.CreateTemp("", "coach-inprocess-stdout-*")
	Expect(err).NotTo(HaveOccurred())
	defer os.Remove(outFile.Name())
	errFile, err := os.CreateTemp("", "coach-inprocess-stderr-*")
	Expect(err).NotTo(HaveOccurred())
	defer os.Remove(errFile.Name())

	exitCode = run(args, outFile, errFile)

	Expect(outFile.Close()).To(Succeed())
	Expect(errFile.Close()).To(Succeed())

	stdout, err = os.ReadFile(outFile.Name())
	Expect(err).NotTo(HaveOccurred())
	stderr, err = os.ReadFile(errFile.Name())
	Expect(err).NotTo(HaveOccurred())
	return stdout, stderr, exitCode
}

var _ = Describe("coach codesignal --project-config", func() {
	When("no --project-config is supplied", func() {
		It("produces a schema-1 report with no project_* keys, regardless of --project-language", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			withoutLanguage, _, exitCodeWithout := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCodeWithout).To(Equal(0))

			withLanguage, _, exitCodeWith := runCoachCodesignalRaw(repo, initialSHA, "--format=json", "--project-language", "typescript")
			Expect(exitCodeWith).To(Equal(0))

			Expect(withLanguage).To(Equal(withoutLanguage), "--project-language with no --project-config must be a silent no-op")

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(withoutLanguage, &document)).To(Succeed())
			Expect(document).To(HaveKey("schema_version"))
			var schemaVersion string
			Expect(json.Unmarshal(document["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("1"))

			Expect(document).NotTo(HaveKey("project_changes"))
			Expect(document).NotTo(HaveKey("project_summary"))
			Expect(document).NotTo(HaveKey("project_coverage"))
			Expect(document).NotTo(HaveKey("project_base_analyzed"))
		})
	})

	// An explicitly empty --project-config is invalid configuration, not
	// "mode disabled". Omitted flag remains the only disabled-mode path.
	When("--project-config is supplied with an empty value", func() {
		It("exits 2, writes nothing to stdout, and writes an actionable message to stderr", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			omitted, _, omittedExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(omittedExit).To(Equal(0))
			Expect(omitted).NotTo(BeEmpty())

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config=", "--format=json")
			Expect(exitCode).To(Equal(2), "explicit empty --project-config must be project_config_invalid; stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout")
			Expect(string(stderr)).NotTo(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring(initialSHA), "stderr must identify the analyzed revision")
		})
	})

	When("--project-config points at a nonexistent file", func() {
		DescribeTable("exits 2, writes nothing to stdout, and writes an actionable message to stderr naming the path and revision, regardless of --format",
			func(formatArg string) {
				repo := newTempGitRepo()
				initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

				stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "missing-project.json", formatArg)

				Expect(exitCode).To(Equal(2))
				Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout, in any --format")
				Expect(string(stderr)).NotTo(BeEmpty())
				Expect(string(stderr)).To(ContainSubstring("missing-project.json"))
				Expect(string(stderr)).To(ContainSubstring(initialSHA))
				Expect(string(stderr)).NotTo(ContainSubstring("exit status"), "stderr must not leak a child process's raw exit status (AC-5)")
				Expect(string(stderr)).NotTo(ContainSubstring("fatal:"), "stderr must not leak git's raw stderr text (AC-5)")
			},
			Entry("--format=json", "--format=json"),
			Entry("--format=text", "--format=text"),
		)
	})

	When("--project-config points at a file that is not valid JSON", func() {
		It("exits 2, writes nothing to stdout, and writes an actionable message to stderr naming the path and revision", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			headSHA := commitFile(repo, "not-json.txt", "not valid json")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "not-json.txt", "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("not-json.txt"))
			Expect(string(stderr)).To(ContainSubstring(headSHA), "stderr must identify HEAD, the analyzed revision, not --base")
		})
	})

	When("--project-config is syntactically valid JSON", func() {
		// Nested roots are required by the multi-module candidate contract: a
		// workspace root may contain a more specific module root. Validation
		// must accept that shape and reach backend dispatch.
		It("accepts nested roots such as . plus a descendant module path", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":[".","services/payments"]}`)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "nested roots must not be rejected as project_config_invalid; got stdout=%s stderr=%s", stdout, stderr)
			Expect(stderr).To(BeEmpty())
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_backend_unavailable"`))
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_config_invalid"`))
		})

		It("accepts two non-dot ancestor/descendant roots", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["services","services/payments"]}`)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "ancestor/descendant roots must not be rejected; got stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_backend_unavailable"`))
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_config_invalid"`))
		})

		It("rejects exact duplicate roots, writing nothing to stdout and an actionable stderr message", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			headSHA := commitFile(repo, "project.json", `{"schema_version":"1","roots":["services/payments","services/payments"]}`)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project.json"))
			Expect(string(stderr)).To(ContainSubstring(headSHA))
		})

		It("rejects an oversized project-config blob, writing nothing to stdout and an actionable stderr message", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			oversized := `{"schema_version":"1","roots":["` + strings.Repeat("a", 1<<20) + `"]}`
			headSHA := commitFile(repo, "project.json", oversized)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project.json"))
			Expect(string(stderr)).To(ContainSubstring(headSHA))
		})
	})

	When("--project-language is not a recognized value", func() {
		It("exits 2 with an actionable message and no stdout, with --project-config also supplied", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--project-language", "rust")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring(`invalid --project-language value "rust"`))
		})

		It("exits 2 with an actionable message and no stdout, with no --project-config supplied", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-language", "rust")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring(`invalid --project-language value "rust"`))
		})
	})
})

var _ = Describe("coach codesignal project-mode exit-code classification", func() {
	var (
		originalLoadProjectConfig     func(string, string, string) (json.RawMessage, error)
		originalResolveProjectBackend func(string) error
	)

	BeforeEach(func() {
		originalLoadProjectConfig = loadProjectConfig
		originalResolveProjectBackend = resolveProjectBackend
	})

	AfterEach(func() {
		loadProjectConfig = originalLoadProjectConfig
		resolveProjectBackend = originalResolveProjectBackend
	})

	When("loadProjectConfig fails with an error that is not a *codesignalcli.ProjectConfigError", func() {
		It("does not classify it as a project-config usage error (exit 2); it falls back to the operational-error path", func() {
			loadProjectConfig = func(string, string, string) (json.RawMessage, error) {
				return nil, errors.New("boom: not a ProjectConfigError")
			}

			stdout, stderr, exitCode := runInProcess("codesignal", "--baseline", "--project-config", "irrelevant")

			Expect(exitCode).NotTo(Equal(2), "an error of an unexpected type must not be classified as project_config_invalid purely because it came from the --project-config call site")
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("boom: not a ProjectConfigError"))
		})
	})

	When("resolveProjectBackend fails with an error that is not a *codesignalcli.ProjectBackendUnavailableError", func() {
		// This asserts only the negative classification (exit code must not
		// be 3). The positive project_backend_unavailable/exit-3 case is
		// covered by "writes a local report and structured diagnostic when
		// the selected backend is unavailable" in
		// project_contract_acceptance_test.go.
		It("does not classify it as project-backend-unavailable (exit 3); it falls back to the operational-error path", func() {
			loadProjectConfig = func(string, string, string) (json.RawMessage, error) {
				return json.RawMessage(`{"schema_version":"1","roots":["."]}`), nil
			}

			resolveProjectBackend = func(string) error {
				return errors.New("boom: not a ProjectBackendUnavailableError")
			}

			stdout, stderr, exitCode := runInProcess("codesignal", "--baseline", "--project-config", "project.json")

			Expect(exitCode).NotTo(Equal(3), "an error of an unexpected type must not be classified as project_backend_unavailable purely because it came from the --project-language call site")
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("boom: not a ProjectBackendUnavailableError"))
		})
	})
})

var _ = Describe("coach codesignal --suggest-project-config flag parsing", func() {
	It("rejects --suggest-project-config supplied more than once", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/dup\n\ngo 1.25\n")

		stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--suggest-project-config")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("--suggest-project-config may only be provided once"))
	})

	It("rejects --output supplied more than once", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/dup\n\ngo 1.25\n")

		stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "a.json", "--output", "b.json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("--output may only be provided once"))
	})

	It("treats a --suggest-project-config value that fails strconv.ParseBool as requested, surfacing the suggestion envelope instead of plain usage text", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/dup\n\ngo 1.25\n")

		stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config=notabool")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_invalid_arguments"), "an unparsable --suggest-project-config value must be classified as requested, not silently ignored")
	})
})
