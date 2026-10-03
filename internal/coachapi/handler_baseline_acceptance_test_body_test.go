package coachapi_test

import (
	"bytes"
	"context"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineAcceptanceTest_completesABaselineViaAgentloopAgainstTheFixtureA_70() {
	var observed *agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = loop
		},
	})

	job := baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
		Ref:       "main",
	})
	_, w := newMemoryFencedWriter(job)
	completion, err := h(context.Background(), job, w)
	Expect(err).NotTo(HaveOccurred(),
		"InsertFindings must mint UUID ids and unique payload_hash values (postgres PK/UNIQUE)")
	Expect(completion).NotTo(BeNil())
	Expect(completion.CommitSHA).To(Equal("local-fixture"))
	Expect(completion.Versions.Analyzer).NotTo(BeEmpty())
	Expect(completion.Versions.Rubrics).To(HaveKey(rubrics.IDHiddenMutationContextualization))
	Expect(completion.Versions.Rubrics).To(HaveKey(rubrics.IDChangeCohesion))

	cap := newCaptureWriter()
	_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
		Ref:       "main",
	}), cap)
	Expect(err).NotTo(HaveOccurred())

	var det, agent int
	uniqKeys := map[string]struct{}{}
	for _, f := range cap.findings {
		Expect(f.ID).NotTo(BeEmpty(), "every finding requires a minted UUID id")
		Expect(f.ID).To(MatchRegexp(uuidShape.String()), "finding id must be UUID-shaped for postgres")
		Expect(f.PayloadHash).NotTo(BeEmpty(), "every finding requires a stable payload_hash")
		Expect(f.Payload).NotTo(BeEmpty())
		key := findingUniqKey(f)
		_, dup := uniqKeys[key]
		Expect(dup).To(BeFalse(), "payload_hash must be unique per (source, rubric_id): %s", key)
		uniqKeys[key] = struct{}{}
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			det++
		case coachapi.FindingSourceAgent:
			agent++
			Expect(f.RubricID).NotTo(BeNil())
			Expect(f.RubricVersion).NotTo(BeNil())
			Expect(f.ModelIdentity).NotTo(BeNil())
		}
	}
	for _, d := range cap.diagnostics {
		Expect(d.ID).NotTo(BeEmpty(), "every diagnostic requires a minted UUID id")
		Expect(d.ID).To(MatchRegexp(uuidShape.String()), "diagnostic id must be UUID-shaped for postgres")
	}
	Expect(det).To(BeNumerically(">=", 1),
		"fixture widget/*.go must produce at least one deterministic codesignal signal")
	Expect(agent).To(BeNumerically(">=", 1),
		"stub gateway should yield at least one source=agent judgment finding")

	Expect(observed).NotTo(BeNil(), "handler must construct an agentloop for the analysis path")
}

func body_handlerBaselineAcceptanceTest_persistsDistinctAgentPayloadHashValuesForMultipl_162() {

	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
	})

	job := baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	})
	_, w := newMemoryFencedWriter(job)
	completion, err := h(context.Background(), job, w)
	Expect(err).NotTo(HaveOccurred(),
		"multi hidden-mutation agent findings must not collide on payload_hash")
	Expect(completion).NotTo(BeNil())

	cap := newCaptureWriter()
	_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	}), cap)
	Expect(err).NotTo(HaveOccurred())

	var hiddenAgentHashes []string
	var detHidden int
	for _, f := range cap.findings {
		Expect(f.ID).To(MatchRegexp(uuidShape.String()))
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			if bytes.Contains(f.Payload, []byte(`"hidden_input_mutation"`)) ||
				bytes.Contains(f.Payload, []byte(`state.hidden_input_mutation`)) {
				detHidden++
			}
		case coachapi.FindingSourceAgent:
			if f.RubricID != nil && *f.RubricID == rubrics.IDHiddenMutationContextualization {
				hiddenAgentHashes = append(hiddenAgentHashes, f.PayloadHash)
			}
		}
	}
	Expect(detHidden).To(BeNumerically(">=", 2),
		"fixture must yield ≥2 deterministic hidden_input_mutation signals")
	Expect(hiddenAgentHashes).To(HaveLen(detHidden),
		"one agent judgment finding per hidden-mutation deterministic signal")
	Expect(hiddenAgentHashes[0]).NotTo(Equal(hiddenAgentHashes[1]),
		"agent payload_hash values for distinct signals must differ")
	uniq := map[string]struct{}{}
	for _, hsh := range hiddenAgentHashes {
		uniq[hsh] = struct{}{}
	}
	Expect(uniq).To(HaveLen(len(hiddenAgentHashes)),
		"all hidden_mutation agent payload_hash values must be unique")
}

func body_handlerBaselineAcceptanceTest_recordsHandlerSourcedSemanticsAnalyzeAndCodesign_218() {
	// Analyze and judgment use separate loops (judgment wall isolation).
	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
		},
	})

	w := newCaptureWriter()
	_, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(observed).NotTo(BeEmpty())

	var names []string
	for _, loop := range observed {
		names = append(names, handlerSourcedNames(loop.Calls())...)
	}
	Expect(names).To(ContainElement(agentloop.ToolSemanticsAnalyze),
		"analysis must go through agentloop.Call(handler, semantics_analyze); no direct pkg/semantics bypass")
	Expect(names).To(ContainElement(agentloop.ToolCodeSignalReport),
		"analysis must go through agentloop.Call(handler, codesignal_report); no direct pkg/codesignal bypass")
	Expect(names).To(ContainElement(rubrics.IDChangeCohesion),
		"change_cohesion must run via agentloop.Call(handler, …)")
	Expect(names).To(ContainElement(rubrics.IDHiddenMutationContextualization),
		"hidden_mutation_contextualization must run via agentloop when deterministic signals exist")
}
