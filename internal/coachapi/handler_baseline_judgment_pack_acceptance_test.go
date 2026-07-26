package coachapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// multiHiddenMutationFixtureRoot builds a temp tree with ≥12 hidden_input_mutation
// signals across ≥3 paths and one hot path with ≥6 signals (Story 1 pack fixture).
func multiHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	// hot.go: 8 pointer-parameter mutations (hot path ≥6).
	hot := `package hot

type S struct {
	A, B, C, D, E, F, G, H string
}

func M1(s *S, v string) { s.A = v }
func M2(s *S, v string) { s.B = v }
func M3(s *S, v string) { s.C = v }
func M4(s *S, v string) { s.D = v }
func M5(s *S, v string) { s.E = v }
func M6(s *S, v string) { s.F = v }
func M7(s *S, v string) { s.G = v }
func M8(s *S, v string) { s.H = v }
`
	// cold_a.go / cold_b.go: 3 mutations each (cross-file merge eligible under affinity).
	coldA := `package colda

type S struct {
	X, Y, Z string
}

func A1(s *S, v string) { s.X = v }
func A2(s *S, v string) { s.Y = v }
func A3(s *S, v string) { s.Z = v }
`
	coldB := `package coldb

type S struct {
	X, Y, Z string
}

func B1(s *S, v string) { s.X = v }
func B2(s *S, v string) { s.Y = v }
func B3(s *S, v string) { s.Z = v }
`
	Expect(os.WriteFile(filepath.Join(root, "hot.go"), []byte(hot), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_a.go"), []byte(coldA), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_b.go"), []byte(coldB), 0o644)).To(Succeed())
	return root
}

// recordingJudgeGateway records every Judge call and optionally delays or
// fails after N successful judgments (budget / packing acceptance).
type recordingJudgeGateway struct {
	inner modelgateway.Gateway

	mu        sync.Mutex
	reqs      []modelgateway.JudgmentRequest
	callCount atomic.Int32

	// delay is applied before each successful Judge (slow fake gateway).
	delay time.Duration

	// failAfterN, when > 0, returns ErrBudgetExceeded-style unavailability after
	// N successful Judge calls (used with short judgment wall via real sleep+wall).
	// When 0, all calls delegate to inner.
	//
	// For pack-budget tests we instead use a short MaxWallTime + delay so the
	// agentloop surfaces agentloop.ErrBudgetExceeded mid-phase.
	failAfterN int
	failErr    error

	// blockAfterN, when > 0, blocks on ctx.Done() for call number > N (1-based)
	// after recording the request. Used to force wall expiry on a specific pack.
	blockAfterN int

	// fixedJudgment, when non-nil, is returned instead of inner for non-blocking calls.
	fixedJudgment json.RawMessage
}

func newRecordingJudgeGateway(inner modelgateway.Gateway) *recordingJudgeGateway {
	if inner == nil {
		inner = modelgateway.NewStubGateway()
	}
	return &recordingJudgeGateway{inner: inner}
}

func (g *recordingJudgeGateway) Judge(ctx context.Context, req modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
	cloned := modelgateway.JudgmentRequest{
		RubricID:      req.RubricID,
		RubricVersion: req.RubricVersion,
		LogicalModel:  req.LogicalModel,
		OutputSchema:  append(json.RawMessage(nil), req.OutputSchema...),
		Messages:      append([]modelgateway.Message(nil), req.Messages...),
	}
	g.mu.Lock()
	g.reqs = append(g.reqs, cloned)
	n := len(g.reqs)
	g.mu.Unlock()
	g.callCount.Add(1)

	if g.failAfterN > 0 && n > g.failAfterN {
		err := g.failErr
		if err == nil {
			err = modelgateway.NewUnavailableError("injected gateway stop after pack budget", nil)
		}
		return modelgateway.JudgmentResponse{}, err
	}
	if g.blockAfterN > 0 && n > g.blockAfterN {
		<-ctx.Done()
		return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", ctx.Err())
	}
	if g.delay > 0 {
		select {
		case <-ctx.Done():
			return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", ctx.Err())
		case <-time.After(g.delay):
		}
	}
	if len(g.fixedJudgment) > 0 {
		return modelgateway.JudgmentResponse{
			JudgmentJSON:   append(json.RawMessage(nil), g.fixedJudgment...),
			LogicalModelID: modelgateway.LogicalModelStub,
		}, nil
	}
	return g.inner.Judge(ctx, req)
}

func (g *recordingJudgeGateway) requests() []modelgateway.JudgmentRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]modelgateway.JudgmentRequest, len(g.reqs))
	copy(out, g.reqs)
	return out
}

func countHiddenMutationToolCalls(loops []*agentloop.Loop) int {
	n := 0
	for _, loop := range loops {
		if loop == nil {
			continue
		}
		for _, c := range loop.Calls() {
			if c.Source == agentloop.CallSourceHandler && c.Name == rubrics.IDHiddenMutationContextualization {
				n++
			}
		}
	}
	return n
}

func countFindingsBySource(findings []coachapi.JobFinding) (det, agent, detHidden, agentHidden int) {
	for _, f := range findings {
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			det++
			if strings.Contains(string(f.Payload), "hidden_input_mutation") ||
				strings.Contains(string(f.Payload), "state.hidden_input_mutation") {
				detHidden++
			}
		case coachapi.FindingSourceAgent:
			agent++
			if f.RubricID != nil && *f.RubricID == rubrics.IDHiddenMutationContextualization {
				agentHidden++
			}
		}
	}
	return det, agent, detHidden, agentHidden
}

var _ = Describe("repo_baseline_scan packed judgment (local-LLM)", func() {
	When("a multi-finding fixture has ≥12 hidden_mutation signals across ≥3 paths with one hot path ≥6", func() {
		It("issues strictly fewer hidden_mutation_contextualization tool calls than finding count (packed judgment)", func() {
			root := multiHiddenMutationFixtureRoot()
			recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

			var observed []*agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          recGW,
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = append(observed, loop)
				},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(BeNumerically(">=", 12),
				"fixture must yield ≥12 deterministic hidden_input_mutation signals; got %d", detHidden)
			Expect(agentHidden).To(Equal(detHidden),
				"one source=agent row per judged hidden-mutation signal")

			// Paths: hot + cold_a + cold_b (assert via payload paths).
			paths := map[string]int{}
			for _, f := range w.findings {
				if f.Source != coachapi.FindingSourceDeterministic {
					continue
				}
				if !strings.Contains(string(f.Payload), "hidden_input_mutation") {
					continue
				}
				var sig struct {
					Path string `json:"path"`
				}
				Expect(json.Unmarshal(f.Payload, &sig)).To(Succeed())
				paths[sig.Path]++
			}
			Expect(len(paths)).To(BeNumerically(">=", 3), "fixture paths: %v", paths)
			var maxPath int
			for _, c := range paths {
				if c > maxPath {
					maxPath = c
				}
			}
			Expect(maxPath).To(BeNumerically(">=", 6), "hot path finding count; paths=%v", paths)

			hmCalls := countHiddenMutationToolCalls(observed)
			Expect(hmCalls).To(BeNumerically(">=", 1))
			Expect(hmCalls).To(BeNumerically("<", detHidden),
				"packed judgment must issue fewer hidden_mutation tool calls than findings; calls=%d findings=%d",
				hmCalls, detHidden)

			// At least one multi-finding pack: a Judge request with batch OutputSchema
			// and ≥2 finding_ref mentions, or a tool call args items len ≥ 2.
			var sawMultiPackArgs bool
			for _, loop := range observed {
				if loop == nil {
					continue
				}
				for _, c := range loop.Calls() {
					if c.Name != rubrics.IDHiddenMutationContextualization {
						continue
					}
					var args struct {
						Items []json.RawMessage `json:"items"`
					}
					if json.Unmarshal(c.Args, &args) == nil && len(args.Items) >= 2 {
						sawMultiPackArgs = true
					}
				}
			}
			Expect(sawMultiPackArgs).To(BeTrue(),
				"at least one multi-finding pack tool call is required")

			// Gateway Judge count for hidden_mutation must also be < finding count.
			hmJudge := 0
			for _, req := range recGW.requests() {
				if req.RubricID == rubrics.IDHiddenMutationContextualization {
					hmJudge++
				}
			}
			Expect(hmJudge).To(BeNumerically("<", detHidden),
				"gateway Judge calls for hidden_mutation must be packed; judges=%d findings=%d",
				hmJudge, detHidden)
		})

		It("embeds span-window evidence in pack args rather than full file content by default", func() {
			root := multiHiddenMutationFixtureRoot()
			// Pad hot.go so full-file content is much larger than a ±15 window.
			hotPath := filepath.Join(root, "hot.go")
			body, err := os.ReadFile(hotPath)
			Expect(err).NotTo(HaveOccurred())
			var pad strings.Builder
			pad.Write(body)
			for i := 0; i < 80; i++ {
				fmt.Fprintf(&pad, "// pad-line-%d-unique-marker-FULLFILE\n", i)
			}
			Expect(os.WriteFile(hotPath, []byte(pad.String()), 0o644)).To(Succeed())

			var observed []*agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = append(observed, loop)
				},
			})

			w := newCaptureWriter()
			_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())

			var checked int
			for _, loop := range observed {
				if loop == nil {
					continue
				}
				for _, c := range loop.Calls() {
					if c.Name != rubrics.IDHiddenMutationContextualization {
						continue
					}
					var args struct {
						Items []struct {
							File struct {
								Content string `json:"content"`
							} `json:"file"`
						} `json:"items"`
						File struct {
							Content string `json:"content"`
						} `json:"file"`
					}
					Expect(json.Unmarshal(c.Args, &args)).To(Succeed())
					contents := []string{}
					for _, it := range args.Items {
						contents = append(contents, it.File.Content)
					}
					if args.File.Content != "" {
						contents = append(contents, args.File.Content)
					}
					for _, content := range contents {
						if content == "" {
							continue
						}
						checked++
						// Span windows are numbered ("  N|..." / "> N|..."); full files are not.
						Expect(content).To(MatchRegexp(`(?m)^[> ]\s*\d+\|`),
							"evidence should be FormatSpanWindow-numbered, not raw full file")
						Expect(content).NotTo(ContainSubstring("pad-line-79-unique-marker-FULLFILE"),
							"default evidence must not embed the entire padded file")
					}
				}
			}
			Expect(checked).To(BeNumerically(">=", 1), "expected at least one pack item with file content")
		})
	})

	When("judgment wall budget is exceeded mid-phase after successful packs", func() {
		It("persists agent findings from completed packs, records a judgment budget diagnostic, keeps deterministic findings, and completes the job", func() {
			root := multiHiddenMutationFixtureRoot()
			recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
			// Each pack Judge sleeps long enough that a short judgment wall
			// allows only the first pack (or first few) before ErrBudgetExceeded.
			recGW.delay = 80 * time.Millisecond

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath:    root,
				SmokeRepoOwner:      "pack-owner",
				SmokeRepoName:       "pack-repo",
				Gateway:             recGW,
				JudgmentMaxWallTime: 100 * time.Millisecond,
				// Small packs → more round-trips so wall trips mid-phase.
				PackConfig: rubrics.PackConfig{MaxFindingsPerJudgmentPack: 2},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred(),
				"judgment budget exceed must complete the job (Story 2 / baseline Story 5 degrade)")
			Expect(completion).NotTo(BeNil())

			det, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(det).To(BeNumerically(">=", 1))
			Expect(detHidden).To(BeNumerically(">=", 12),
				"deterministic hidden-mutation findings must remain complete")
			Expect(agentHidden).To(BeNumerically(">=", 1),
				"at least one pack's agent findings must already be persisted before budget exceed")
			Expect(agentHidden).To(BeNumerically("<", detHidden),
				"budget exceed mid-phase must leave some findings unjudged; agent=%d det=%d",
				agentHidden, detHidden)

			var sawBudgetDiag bool
			for _, d := range w.diagnostics {
				msg := strings.ToLower(d.Message)
				scope := strings.ToLower(d.Scope)
				if strings.Contains(msg, "judgment_budget_exceeded") ||
					(strings.Contains(msg, "judged=") && strings.Contains(msg, "remaining=")) ||
					(strings.Contains(scope, "judgment") && strings.Contains(scope, "budget")) {
					sawBudgetDiag = true
					// Prefer stable counts when present.
					if strings.Contains(d.Message, "judged=") {
						Expect(d.Message).To(ContainSubstring("remaining="))
					}
				}
			}
			Expect(sawBudgetDiag).To(BeTrue(),
				"must record judgment budget diagnostic with judged/remaining; got %#v", w.diagnostics)
		})

		It("records judgment_budget_exceeded when the wall expires during the sole pack's in-flight Judge", func() {
			// Story 2 gap: wall death mid-Judge on the last/only pack must not
			// soft-degrade to gateway-unavailable diagnostics without a budget diagnostic.
			root := multiHiddenMutationFixtureRoot()
			recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
			recGW.delay = 200 * time.Millisecond

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath:           root,
				SmokeRepoOwner:             "pack-owner",
				SmokeRepoName:              "pack-repo",
				Gateway:                    recGW,
				JudgmentMaxWallTime:        50 * time.Millisecond,
				MaxHiddenMutationJudgments: -1,
				// One multi-item pack: high affinity threshold avoids path-dedicated splits
				// so wall can only die mid-Judge (no pack N+1 checkWall).
				PackConfig: rubrics.PackConfig{
					MaxFindingsPerJudgmentPack:      50,
					JudgmentFileAffinityMinFindings: 100,
				},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(BeNumerically(">=", 12))
			Expect(agentHidden).To(Equal(0),
				"sole pack wall death mid-Judge must not invent agent rows")

			var budgetMsg string
			for _, d := range w.diagnostics {
				if strings.Contains(d.Message, "judgment_budget_exceeded") ||
					d.Scope == "judgment_budget" {
					budgetMsg = d.Message
					break
				}
			}
			Expect(budgetMsg).NotTo(BeEmpty(),
				"sole-pack wall expiry must emit judgment_budget_exceeded; got %#v", w.diagnostics)
			Expect(budgetMsg).To(ContainSubstring("judged=0"))
			Expect(budgetMsg).To(ContainSubstring("remaining="))
		})

		It("counts judged= as source=agent findings, not diagnostics-only pack items", func() {
			// Pack 1 returns an empty batch (all items → diagnostics, 0 agent rows).
			// Pack 2 blocks until wall cancels. judged must be 0, not pack-1 item count.
			root := multiHiddenMutationFixtureRoot()
			recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
			recGW.fixedJudgment = json.RawMessage(`{"items":[]}`)
			recGW.blockAfterN = 1

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath:    root,
				SmokeRepoOwner:      "pack-owner",
				SmokeRepoName:       "pack-repo",
				Gateway:             recGW,
				JudgmentMaxWallTime: 80 * time.Millisecond,
				PackConfig:          rubrics.PackConfig{MaxFindingsPerJudgmentPack: 4},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, _, agentHidden := countFindingsBySource(w.findings)
			Expect(agentHidden).To(Equal(0))

			var budgetMsg string
			for _, d := range w.diagnostics {
				if strings.Contains(d.Message, "judgment_budget_exceeded") ||
					d.Scope == "judgment_budget" {
					budgetMsg = d.Message
					break
				}
			}
			Expect(budgetMsg).NotTo(BeEmpty(),
				"expected judgment_budget_exceeded; got %#v", w.diagnostics)
			Expect(budgetMsg).To(ContainSubstring("judged=0"),
				"diagnostics-only pack must not inflate judged=; got %q", budgetMsg)
		})
	})

	When("the parent context is canceled during packed judgment", func() {
		It("still aborts without complete-as-success", func() {
			root := multiHiddenMutationFixtureRoot()
			ctx, cancel := context.WithCancel(context.Background())

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ConfigureLoop: func(loop *agentloop.Loop) {
					Expect(loop.Register(agentloop.ToolSpec{
						Name: rubrics.IDHiddenMutationContextualization,
						Handler: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
							cancel()
							return nil, context.Canceled
						},
					})).To(Succeed())
				},
			})

			completion, err := h(ctx, baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	When("judgment uses a separate wall budget from analyze", func() {
		It("applies JudgmentMaxWallTime (default 10m) on the judgment loop so analyze time does not alone exhaust judgment budget", func() {
			root := multiHiddenMutationFixtureRoot()

			var observed []*agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          modelgateway.NewStubGateway(),
				// Explicit non-default to prove config wiring (zero → 10m covered below).
				JudgmentMaxWallTime: 7 * time.Minute,
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = append(observed, loop)
				},
			})

			w := newCaptureWriter()
			_, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(observed).NotTo(BeEmpty())

			// Judgment loop is the one that recorded hidden_mutation calls and
			// must carry JudgmentMaxWallTime (not the residual analyze wall).
			var judgmentLoop *agentloop.Loop
			for _, loop := range observed {
				for _, c := range loop.Calls() {
					if c.Name == rubrics.IDHiddenMutationContextualization {
						judgmentLoop = loop
						break
					}
				}
			}
			Expect(judgmentLoop).NotTo(BeNil(), "judgment loop must be observed")
			Expect(judgmentLoop.Budget().MaxWallTime).To(Equal(7*time.Minute),
				"judgment loop wall must come from JudgmentMaxWallTime, not shared analyze residual")

			// Default (zero) → 10 minutes.
			var observedDefault []*agentloop.Loop
			hDefault := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop: func(loop *agentloop.Loop) {
					observedDefault = append(observedDefault, loop)
				},
			})
			_, err = hDefault(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), newCaptureWriter())
			Expect(err).NotTo(HaveOccurred())
			var jDefault *agentloop.Loop
			for _, loop := range observedDefault {
				for _, c := range loop.Calls() {
					if c.Name == rubrics.IDHiddenMutationContextualization {
						jDefault = loop
						break
					}
				}
			}
			Expect(jDefault).NotTo(BeNil())
			Expect(jDefault.Budget().MaxWallTime).To(Equal(10*time.Minute),
				"zero JudgmentMaxWallTime must default to 10m")
		})
	})
})
