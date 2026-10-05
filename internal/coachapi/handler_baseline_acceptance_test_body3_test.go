package coachapi_test

import (
	"context"

	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAndJudgme_594() {
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway: modelgateway.NewStubGateway(modelgateway.StubOptions{
			JudgeErr: modelgateway.NewUnavailableError("gateway down", nil),
		}),
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(), "judgment degrade must not fail the job (Story 5)")
	Expect(completion).NotTo(BeNil())

	allFindings := append([]coachapi.JobFinding{}, w.findings...)
	allFindings = append(allFindings, completion.Findings...)
	var det int
	for _, f := range allFindings {
		if f.Source == coachapi.FindingSourceDeterministic {
			det++
		}
		Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
			"unavailable gateway must not produce source=agent findings")
	}
	Expect(det).To(BeNumerically(">=", 1))

	allDiags := append([]coachapi.JobDiagnostic{}, w.diagnostics...)
	allDiags = append(allDiags, completion.Diagnostics...)
	Expect(allDiags).NotTo(BeEmpty(), "judgment degrade must record JobDiagnostic entries")
	var sawRubricScope bool
	for _, d := range allDiags {
		if len(d.Scope) > 0 && (d.Scope == "rubric:"+rubrics.IDHiddenMutationContextualization ||
			d.Scope == "rubric:"+rubrics.IDChangeCohesion ||
			len(d.Scope) >= 7 && d.Scope[:7] == "rubric:") {
			sawRubricScope = true
		}
	}
	Expect(sawRubricScope).To(BeTrue(), "diagnostics should scope to rubric:* from degrade envelopes")
}

func body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAndSchema_640() {
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway: modelgateway.NewStubGateway(modelgateway.StubOptions{
			JudgeErr: modelgateway.NewValidationError("judgment missing required field: confidence"),
		}),
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"schema-validation degrade must not fail the job (Story 5); got %v", err)
	Expect(completion).NotTo(BeNil())

	var det int
	for _, f := range w.findings {
		if f.Source == coachapi.FindingSourceDeterministic {
			det++
		}
		Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
			"schema-invalid judgments must not become source=agent findings")
	}
	Expect(det).To(BeNumerically(">=", 1),
		"deterministic findings must survive schema-validation degrade")
	Expect(w.diagnostics).NotTo(BeEmpty(),
		"schema-validation degrade must record JobDiagnostic entries")

	var sawSchemaDiag bool
	for _, d := range w.diagnostics {
		if strings.Contains(strings.ToLower(d.Message), "schema") ||
			strings.Contains(d.Message, "confidence") ||
			strings.Contains(d.Message, "validation") ||
			strings.HasPrefix(d.Scope, "rubric:") {
			sawSchemaDiag = true
		}
	}
	Expect(sawSchemaDiag).To(BeTrue(),
		"diagnostics should describe schema/validation failure from rubric degrade envelopes; got %#v", w.diagnostics)
}
