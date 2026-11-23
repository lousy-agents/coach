package main

import (
	"io/fs"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal reads the project policy from disk: a canonical policy that cannot be read", func() {
	DescribeTable("names the cause once",
		func(reported []codesignal.Diagnostic, wantUnusableOnDisk int) {
			ports := defaultScanPorts()
			ports.selectOnDisk = func(string, string) (projectconfig.DiskSelection, error) {
				return projectconfig.DiskSelection{}, fs.ErrPermission
			}

			prepared, err := loadPreparedProject(GinkgoT().TempDir(), "rev", policyref.Ref{}, "go", reported, ports)

			Expect(err).NotTo(HaveOccurred())
			Expect(diagnosticsOfKind(prepared.diagnostics, "project_policy_unusable_on_disk")).To(HaveLen(wantUnusableOnDisk))
			Expect(diagnosticsOfKind(prepared.diagnostics, "project_config_unusable")).To(HaveLen(len(diagnosticsOfKind(reported, "project_config_unusable"))))
		},
		Entry("discovery reported the canonical path as unusable",
			[]codesignal.Diagnostic{{Kind: "project_config_unusable", Path: projectconfig.CanonicalPath}}, 0),
		Entry("discovery reported nothing about the path",
			nil, 1),
		Entry("discovery reported another path as unusable",
			[]codesignal.Diagnostic{{Kind: "project_config_unusable", Path: "elsewhere.json"}}, 1),
		Entry("discovery reported the canonical path under another kind",
			[]codesignal.Diagnostic{{Kind: "project_config_deprecated", Path: projectconfig.CanonicalPath}}, 1),
	)
})
