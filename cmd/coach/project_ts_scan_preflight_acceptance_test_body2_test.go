package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func body_projectTsScanPreflightAcceptanceTest_codesignalcliCheckProjectReadinessNeverGatesOrWa_265() {
	It("has more than one supported Node major, so the table below cannot silently degrade to exercising just one", func() {
		Expect(len(tstoolchain.SupportedNodeMajors)).To(BeNumerically(">", 1), "codesignalcli.SupportedNodeMajors=%v", tstoolchain.SupportedNodeMajors)
	})

	tableArgs := []any{func(major int) {
		body_projectTsScanPreflightAcceptanceTest_270(major)
	}}
	for _, major := range tstoolchain.SupportedNodeMajors {
		tableArgs = append(tableArgs, Entry(fmt.Sprintf("Node major %d", major), major))
	}

	DescribeTable("passes node/runtime with no gap, no warning, and no runtime next action, and a real scan proceeds past the Node boundary to fail only on the later missing-compiler gap", tableArgs...)
}
