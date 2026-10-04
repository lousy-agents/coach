package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_acceptanceTest_reportsMissingHeadResultsForAddedAndModifiedFile_179() {
	for _, status := range []codesignal.ChangeStatus{"added", "modified"} {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: string(status) + ".go", Status: status}}})
		Expect(diagnostic(report, "missing_head_result", string(status)+".go")).NotTo(BeNil())
	}
}
