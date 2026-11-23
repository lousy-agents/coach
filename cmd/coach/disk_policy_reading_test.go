package main

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("judging the policy a scan named against the working tree", func() {
	named := policyref.Named("policies/custom.json")
	oversize := &projectconfig.DiskPolicyRefusal{Path: named.Path, Reason: "larger than the policy size limit"}
	oversizeOnDisk := func(string, string) (projectconfig.DiskSelection, error) {
		return projectconfig.DiskSelection{}, oversize
	}

	It("returns a failure of the loader that is not about the policy as it is, and does not word it as a refusal of the file", func() {
		gitFailure := errors.New("git: could not run")
		ports := defaultScanPorts()
		ports.selectOnDisk = oversizeOnDisk
		ports.loadExplicit = func(string, string, string) (json.RawMessage, error) { return nil, gitFailure }

		_, err := judgeNamedOnDisk("dir", "rev", named, ports)

		Expect(err).To(BeIdenticalTo(gitFailure))
		var configErr *projectconfig.ConfigError
		Expect(errors.As(err, &configErr)).To(BeFalse())
	})

	It("words the refusal for the disk when the loader finds nothing wrong with the committed copy", func() {
		ports := defaultScanPorts()
		ports.selectOnDisk = oversizeOnDisk
		ports.loadExplicit = func(string, string, string) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

		_, err := judgeNamedOnDisk("dir", "rev", named, ports)

		var configErr *projectconfig.ConfigError
		Expect(errors.As(err, &configErr)).To(BeTrue())
		Expect(configErr.Message).To(ContainSubstring("cannot be used on disk"))
	})
})
