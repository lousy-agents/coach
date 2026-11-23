package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

var _ = Describe("coach codesignal reads the project policy from disk: no backend can run the policy", func() {
	for _, flags := range [][]string{{"--baseline"}, {"--baseline", "--worktree"}} {
		It("names the canonical policy on disk, not the committed root project.json, and drops that file's deprecation under "+flags[len(flags)-1], func() {
			repo, _ := diskPolicyRepo(scopePolicyFile{legacyConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDiskCreatingParents(repo, coachConfigPath, goLayerPolicyConfigJSON)
			ports := defaultScanPorts()
			ports.resolveBackend = func(string) error {
				return &codesignalcli.ProjectBackendUnavailableError{Message: "no backend for this language (project_backend_unavailable)"}
			}
			original, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chdir(repo)).To(Succeed())
			defer func() { Expect(os.Chdir(original)).To(Succeed()) }()

			stdout, stderr, exitCode := runInProcessWith(ports, append(append([]string{"codesignal"}, flags...), "--format=json")...)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)
			Expect(diagnosticsOfKind(report.Diagnostics, "project_backend_unavailable")).To(ConsistOf(HaveField("Path", coachConfigPath)))
			Expect(diagnosticsOfKind(report.Diagnostics, "project_config_deprecated")).To(BeEmpty())
		})
	}
})
