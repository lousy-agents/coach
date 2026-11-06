package main

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("loading the project for a TypeScript scan whose policy cannot be loaded", func() {
	DescribeTable("hands the readiness check the policy with who chose it",
		func(source policyref.Ref, wantNamedPath string, wantNamed bool) {
			repo := commitScannableGoRepo()
			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			failingLoad := func(string, string, string) (json.RawMessage, error) {
				return nil, &projectconfig.ConfigError{Kind: projectconfig.KindUnreadable, Message: "the policy cannot be read"}
			}
			ports := defaultScanPorts()
			ports.loadExplicit = failingLoad
			ports.loadDiscovered = failingLoad

			_, loadErr := loadPreparedProject(repo, revision, source, "typescript", nil, ports)

			var withReadiness *tssetup.ProjectConfigErrorWithReadiness
			Expect(errors.As(loadErr, &withReadiness)).To(BeTrue(), "the failure carries the readiness snapshot: %v", loadErr)
			namedPath, named := withReadiness.Policy.NamedPath()
			Expect(named).To(Equal(wantNamed))
			Expect(namedPath).To(Equal(wantNamedPath))
		},
		Entry("a policy named with --project-config", policyref.Named(".lousy-agents/coach.json"), ".lousy-agents/coach.json", true),
		Entry("a policy the scan discovered", policyref.Discovered(".lousy-agents/coach.json"), "", false),
	)
})

var _ = Describe("loading the project when no backend can run the policy", func() {
	It("keeps the diagnostics that came with the policy and adds why the backend is unavailable", func() {
		ports := defaultScanPorts()
		ports.loadDiscovered = func(string, string, string) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
		ports.resolveBackend = func(string) error {
			return &codesignalcli.ProjectBackendUnavailableError{Message: "not here"}
		}
		earlier := codesignal.Diagnostic{Kind: "project_config_deprecated", Path: "project.json"}

		prepared, err := loadPreparedProject("", "rev", policyref.Discovered("project.json"), "go", []codesignal.Diagnostic{earlier}, ports)

		Expect(err).NotTo(HaveOccurred())
		Expect(prepared.analysis).To(BeNil())
		Expect(prepared.diagnostics).To(Equal([]codesignal.Diagnostic{
			earlier,
			{Kind: "project_backend_unavailable", Path: "project.json", Message: "not here"},
		}))
	})
})
