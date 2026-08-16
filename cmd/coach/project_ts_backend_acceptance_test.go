package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// fakeTSSidecarBinary holds the compiled testdata/fake_ts_sidecar bytes,
// built lazily on first use via buildFakeTSSidecarOnce -- cmd/coach's suite
// already defines its one allowed BeforeSuite node in
// acceptance_suite_test.go (Ginkgo permits only one), so this cannot add a
// second BeforeSuite the way pkg/projectmodel/ts_sidecar_acceptance_test.go
// does.
var (
	fakeTSSidecarBinary    []byte
	buildFakeTSSidecarOnce sync.Once
)

func ensureFakeTSSidecarBuilt() {
	buildFakeTSSidecarOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fake-ts-project-sidecar-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)

		binPath := filepath.Join(dir, "fake-ts-project-sidecar")
		build := exec.Command("go", "build", "-o", binPath, "./testdata/fake_ts_sidecar")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "building fake ts project sidecar: %s", output)

		data, err := os.ReadFile(binPath)
		Expect(err).NotTo(HaveOccurred())
		fakeTSSidecarBinary = data
	})
}

// installFakeTSSidecar writes the compiled fake sidecar binary at the fixed
// repository-relative path (js/semantics/bin/coach-ts-project-sidecar) the
// real tsProjectBackend resolves against --project-config's checkout dir, so
// the CLI's real filesystem-path resolution is exercised end to end rather
// than through a test-only injection seam.
func installFakeTSSidecar(repo string) {
	ensureFakeTSSidecarBuilt()
	binDir := filepath.Join(repo, "js", "semantics", "bin")
	Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(binDir, "coach-ts-project-sidecar"), fakeTSSidecarBinary, 0o755)).To(Succeed())
}

// tsHandlersWithoutMarker and tsHandlersWithMarker let a test control,
// purely via committed file content, whether testdata/fake_ts_sidecar
// reports its one fixed forbidden-layer edge.
const tsHandlersWithoutMarker = "export function use(): string {\n  return 'no import here';\n}\n"
const tsHandlersWithMarker = "// LAYER_VIOLATION_MARKER\nexport function use(): string {\n  return 'no import here';\n}\n"
const tsDbFile = "export const Name = 'db';\n"

var _ = Describe("coach codesignal --project-config with the real TypeScript project-language backend", func() {
	When("--project-language typescript is selected but no sidecar binary is installed", func() {
		It("stays exit 0 and reports project_backend_unavailable via ProjectCoverage.Diagnostics instead of exit 3", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsHandlersWithMarker)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(BeEmpty(), "a missing sidecar must never fabricate a layer violation")
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse())

			var found bool
			for _, diag := range report.ProjectCoverage.Diagnostics {
				if diag.Code == projectmodel.DiagBackendUnavailable {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
		})
	})

	When("--baseline is run with a working sidecar that reports one eligible forbidden layer edge", func() {
		It("emits exactly one architecture.layer_violation ProjectChange tagged with language typescript", func() {
			repo := newTempGitRepo()
			installFakeTSSidecar(repo)
			commitFile(repo, "pkg/db/d.ts", tsDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsHandlersWithMarker)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.Kind).To(Equal("architecture.layer_violation"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("language", "typescript"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
			Expect(change.PrimaryAnchor.Path).To(Equal("pkg/handlers/h.ts"))

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
		})
	})

	When("--baseline is run with a working sidecar whose reported edges never cross a configured layer boundary", func() {
		It("emits zero ProjectChanges (negative control: real end-to-end pipeline stays silent)", func() {
			repo := newTempGitRepo()
			installFakeTSSidecar(repo)
			commitFile(repo, "pkg/db/d.ts", tsDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsHandlersWithoutMarker)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(BeEmpty())
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
		})
	})

	When("diff mode introduces a forbidden TypeScript layer edge that did not exist at base", func() {
		It("builds distinct head and base project models and classifies the ProjectChange as lifecycle introduced", func() {
			repo := newTempGitRepo()
			installFakeTSSidecar(repo)
			commitFile(repo, "pkg/db/d.ts", tsDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsHandlersWithoutMarker)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/h.ts", tsHandlersWithMarker)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "head-side sidecar call must have succeeded")
		})
	})

	When("--project-config is invalid at the selected revision and --project-language is typescript", func() {
		It("still exits 2 with project_config_invalid, never reaching backend dispatch", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsDbFile)
			commitFile(repo, "project.json", "not valid json")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())
			Expect(string(stdout)).To(ContainSubstring(`"kind":"project_config_invalid"`))
		})
	})
})
