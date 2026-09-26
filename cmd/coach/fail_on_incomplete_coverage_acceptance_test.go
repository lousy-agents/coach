package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

var _ = Describe("coach codesignal --fail-on-incomplete-coverage", func() {
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

	// Seal the seams to simulate a backend that is unavailable (the "rust"
	// backend never ships) so these specs run without real project analysis.
	backendUnavailable := func() {
		loadProjectConfig = func(string, string, string) (json.RawMessage, error) {
			return json.RawMessage(`{"schema_version":"1","roots":["."]}`), nil
		}
		resolveProjectBackend = func(string) error {
			return &codesignalcli.ProjectBackendUnavailableError{
				Message: `coach codesignal: no project-analysis backend is available for language "rust" yet (project_backend_unavailable)`,
			}
		}
	}

	expectReportWithBackendUnavailableDiagnostic := func(stdout []byte) {
		var report struct {
			SchemaVersion string `json:"schema_version"`
			Diagnostics   []struct {
				Kind string `json:"kind"`
			} `json:"diagnostics"`
		}
		Expect(json.Unmarshal(stdout, &report)).To(Succeed())
		Expect(report.SchemaVersion).To(Equal("1"))
		Expect(report.Diagnostics).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Kind": Equal("project_backend_unavailable"),
		})))
	}

	When("the project backend is unavailable and --fail-on-incomplete-coverage is supplied", func() {
		It("exits 3, writes the report to stdout, and emits project_backend_unavailable in diagnostics", func() {
			backendUnavailable()

			stdout, stderr, exitCode := runInProcess("codesignal", "--baseline", "--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3))
			Expect(stderr).To(BeEmpty())
			expectReportWithBackendUnavailableDiagnostic(stdout)
		})
	})

	When("--help is requested", func() {
		It("lists --fail-on-incomplete-coverage in the flag documentation and scan synopsis", func() {
			stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("fail-on-incomplete-coverage"))
			Expect(string(stdout)).To(ContainSubstring("[--fail-on-incomplete-coverage]"))
		})
	})
})

// The specs below exercise AC-EVD-7's model/bypass branch (exit 3 on analyzed
// model incompleteness) and the reachability-isolation guarantee (exit 0 when
// only reachability is incomplete) using the real TypeScript sidecar.
var _ = Describe("coach codesignal --fail-on-incomplete-coverage with real TypeScript backend (AC-EVD-7)", func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("the tsRootScopeGapTSConfigJSON fixture produces incomplete model coverage", Label("ts-project-backend"), func() {
		var repo string

		BeforeEach(func() {
			repo = newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)
		})

		It("exits 0 without the flag, report still emitted with model=incomplete on stdout", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
			report := decodeCoachReport(stdout)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
		})

		It("exits 3 with --fail-on-incomplete-coverage, report still emitted on stdout with model=incomplete", Label("ts-project-backend"), func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stderr).To(BeEmpty())
			report := decodeCoachReport(stdout)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
		})
	})

	When("a routine reachability gap produces no model or bypass incompleteness", Label("ts-project-backend"), func() {
		It("exits 0 with --fail-on-incomplete-coverage because reachability incompleteness alone does not count (AC-EVD-7)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
			commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
			commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Reachability).NotTo(Equal("complete"))
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("complete"))
			Expect(report.ProjectProvenance.Head.Coverage.Bypass).To(Equal("not_requested"))
		})
	})
})
