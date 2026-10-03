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

func body_ruleReactOrchestrationAcceptanceTest_shallMarkTheBaseEvidenceResolvedAndTheHeadEviden_928() {
	base := analyzeTSXForCodesignal("WorkspacePage.tsx", reactOrchestrationRuleP1)
	head := analyzeTSXForCodesignal("WorkspacePage.tsx", reactOrchestrationRuleP1WithDraft)

	report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "WorkspacePage.tsx", Status: "modified", Base: base, Head: head,
	}}})

	signals := signalsByRule(report, reactOrchestrationRuleID)
	Expect(signals).To(HaveLen(2), "expected a resolved base signal and an introduced head signal (distinct evidence keys)")

	byLife := map[codesignal.Lifecycle]codesignal.Signal{}
	for _, s := range signals {
		byLife[s.Lifecycle] = s
	}
	Expect(byLife[codesignal.Lifecycle("resolved")].Evidence).To(Equal(reactOrchestrationEvidenceP1))
	Expect(byLife[codesignal.Lifecycle("introduced")].Evidence).To(Equal(reactOrchestrationEvidenceL1Head))
}
