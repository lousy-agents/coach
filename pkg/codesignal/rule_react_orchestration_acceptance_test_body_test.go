package codesignal_test

import (
	"context"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_ruleReactOrchestrationAcceptanceTest_silentFixturesShallEmitNoReactComponentOrchestra_809(path, source string, language semantics.Language) {
	var head *semantics.Result
	if language == semantics.LanguageGo {
		analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
		Expect(err).NotTo(HaveOccurred())
		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path: path, Language: semantics.LanguageGo, Content: []byte(source),
		})
		Expect(err).NotTo(HaveOccurred())
		head = result
	} else {
		head = analyzeTSXForCodesignal(path, source)
	}

	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: path, Status: "modified", Head: head,
	}}})
	Expect(signalsByRule(report, reactOrchestrationRuleID)).To(BeEmpty())
}
