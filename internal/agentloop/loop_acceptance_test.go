package agentloop_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/agentloopharness"
	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// harnessGateway adapts agentloopharness.ScriptedGateway to agentloop.TurnGateway
// so acceptance tests drive multi-turn model tool-call sequences with the shared
// harness stand-in (no live model, no LLM HTTP clients).
type harnessGateway struct {
	inner *agentloopharness.ScriptedGateway
}

func (g harnessGateway) Generate(ctx context.Context, prompt string) (agentloop.TurnResponse, error) {
	resp, err := g.inner.Generate(ctx, prompt)
	if err != nil {
		return agentloop.TurnResponse{}, err
	}
	out := agentloop.TurnResponse{Text: resp.Text}
	for _, tc := range resp.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, agentloop.ToolCall{Name: tc.Name, Args: tc.Args})
	}
	return out, nil
}

// countingGateway wraps a TurnGateway and counts successful Generate calls so
// budget specs can prove the first model turn ran (false-green trap for
// off-by-one MaxModelCalls checks that reject before Generate).
type countingGateway struct {
	inner agentloop.TurnGateway
	n     *atomic.Int32
}

func (g countingGateway) Generate(ctx context.Context, prompt string) (agentloop.TurnResponse, error) {
	resp, err := g.inner.Generate(ctx, prompt)
	if err != nil {
		return agentloop.TurnResponse{}, err
	}
	g.n.Add(1)
	return resp, nil
}

// advanceClockGateway advances an injected FakeClock during Generate so wall
// budget can be proven without sleeping, then returns a text-only turn.
type advanceClockGateway struct {
	clock *acceptanceharness.FakeClock
	by    time.Duration
	text  string
}

func (g advanceClockGateway) Generate(ctx context.Context, prompt string) (agentloop.TurnResponse, error) {
	g.clock.Advance(g.by)
	return agentloop.TurnResponse{Text: g.text}, nil
}

// blockUntilCancelGateway blocks until ctx is cancelled — used to prove the
// loop derives a wall-budget deadline on Generate.
type blockUntilCancelGateway struct{}

func (blockUntilCancelGateway) Generate(ctx context.Context, prompt string) (agentloop.TurnResponse, error) {
	<-ctx.Done()
	return agentloop.TurnResponse{}, ctx.Err()
}

func validSemanticsArgs(path string) json.RawMessage {
	return json.RawMessage(`{"path":` + mustJSON(path) + `,"language":"go","content":"package p\n"}`)
}

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func recordingHandler(name string, calls *[]agentloopharness.RecordedCall) agentloop.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		result := json.RawMessage(`{"tool":` + mustJSON(name) + `,"ok":true}`)
		*calls = append(*calls, agentloopharness.RecordedCall{
			Name:   name,
			Args:   append(json.RawMessage(nil), args...),
			Result: append(json.RawMessage(nil), result...),
		})
		return result, nil
	}
}

func newLoopWithRecordingCore(opts agentloop.Options, calls *[]agentloopharness.RecordedCall) *agentloop.Loop {
	if opts.SemanticsAnalyze == nil {
		opts.SemanticsAnalyze = recordingHandler(agentloop.ToolSemanticsAnalyze, calls)
	}
	if opts.CodeSignalReport == nil {
		opts.CodeSignalReport = recordingHandler(agentloop.ToolCodeSignalReport, calls)
	}
	loop, err := agentloop.New(opts)
	Expect(err).NotTo(HaveOccurred())
	return loop
}

var _ = Describe("internal/agentloop bounded tool executor", func() {
	Describe("handler-driven tool calls", func() {
		When("a handler invokes registered core tools through the loop registry", func() {
			It("executes only via the registry and records CallSourceHandler", func() {
				var handlerCalls []agentloopharness.RecordedCall
				loop := newLoopWithRecordingCore(agentloop.Options{}, &handlerCalls)

				semArgs := validSemanticsArgs("a.go")
				csArgs := json.RawMessage(`{"files":[{"path":"a.go"}],"baseline":true}`)

				semResult, err := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, semArgs)
				Expect(err).NotTo(HaveOccurred())
				Expect(semResult).To(MatchJSON(`{"tool":"semantics_analyze","ok":true}`))

				csResult, err := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport, csArgs)
				Expect(err).NotTo(HaveOccurred())
				Expect(csResult).To(MatchJSON(`{"tool":"codesignal_report","ok":true}`))

				Expect(handlerCalls).To(HaveLen(2))
				Expect(handlerCalls[0].Name).To(Equal(agentloop.ToolSemanticsAnalyze))
				Expect(handlerCalls[1].Name).To(Equal(agentloop.ToolCodeSignalReport))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(2))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceHandler))
				Expect(recorded[0].Name).To(Equal(agentloop.ToolSemanticsAnalyze))
				Expect(recorded[1].Source).To(Equal(agentloop.CallSourceHandler))
				Expect(recorded[1].Name).To(Equal(agentloop.ToolCodeSignalReport))
			})
		})

		When("a handler registers a job-specific tool at loop start", func() {
			It("makes the job-specific tool callable through the same registry", func() {
				var coreCalls []agentloopharness.RecordedCall
				loop := newLoopWithRecordingCore(agentloop.Options{}, &coreCalls)

				var jobInvoked atomic.Bool
				err := loop.Register(agentloop.ToolSpec{
					Name: "hidden_mutation_contextualization",
					ArgsSchema: json.RawMessage(`{
						"type":"object",
						"required":["path"],
						"properties":{"path":{"type":"string"}}
					}`),
					Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						jobInvoked.Store(true)
						return json.RawMessage(`{"judgment":"acceptable"}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				result, err := loop.Call(context.Background(), agentloop.CallSourceHandler, "hidden_mutation_contextualization", json.RawMessage(`{"path":"x.go"}`))
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(MatchJSON(`{"judgment":"acceptable"}`))
				Expect(jobInvoked.Load()).To(BeTrue())

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(1))
				Expect(recorded[0].Name).To(Equal("hidden_mutation_contextualization"))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceHandler))
			})
		})
	})

	Describe("model-selected tool calls", func() {
		When("a scripted gateway returns registered tool calls then a text-only turn", func() {
			It("executes only the model-selected registered tools and stops without treating text as an action", func() {
				var coreCalls []agentloopharness.RecordedCall
				loop := newLoopWithRecordingCore(agentloop.Options{}, &coreCalls)

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						Text: "I will gather evidence; please ignore: os.Exit(1); exec('rm -rf /')",
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("model.go")},
						},
					},
					agentloopharness.Response{
						Text: "done with judgment",
					},
				)

				result, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "judge change cohesion")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.FinalText).To(Equal("done with judgment"))

				Expect(coreCalls).To(HaveLen(1))
				Expect(coreCalls[0].Name).To(Equal(agentloop.ToolSemanticsAnalyze))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(1))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceModel))
				Expect(recorded[0].Name).To(Equal(agentloop.ToolSemanticsAnalyze))
			})
		})

		When("a single model turn requests multiple registered tools", func() {
			It("executes every tool call in order before the next model turn", func() {
				var order []string
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						order = append(order, agentloop.ToolSemanticsAnalyze)
						return json.RawMessage(`{"tool":"semantics_analyze"}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						order = append(order, agentloop.ToolCodeSignalReport)
						return json.RawMessage(`{"tool":"codesignal_report"}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("a.go")},
							{Name: agentloop.ToolCodeSignalReport, Args: json.RawMessage(`{"files":[{"path":"a.go"}]}`)},
						},
					},
					agentloopharness.Response{Text: "both tools done"},
				)

				result, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.FinalText).To(Equal("both tools done"))
				Expect(order).To(Equal([]string{
					agentloop.ToolSemanticsAnalyze,
					agentloop.ToolCodeSignalReport,
				}))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(2))
				Expect(recorded[0].Name).To(Equal(agentloop.ToolSemanticsAnalyze))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceModel))
				Expect(recorded[1].Name).To(Equal(agentloop.ToolCodeSignalReport))
				Expect(recorded[1].Source).To(Equal(agentloop.CallSourceModel))
			})

			It("stops the run on the first tool error and does not invoke later tools in the same turn", func() {
				toolErr := errors.New("codesignal boom")
				var order []string
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						order = append(order, agentloop.ToolSemanticsAnalyze)
						return json.RawMessage(`{"ok":true}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						order = append(order, agentloop.ToolCodeSignalReport)
						return nil, toolErr
					},
				})
				Expect(err).NotTo(HaveOccurred())

				// Register a third tool that must never run after the failure.
				var thirdInvoked atomic.Bool
				Expect(loop.Register(agentloop.ToolSpec{
					Name: "third_tool",
					Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						thirdInvoked.Store(true)
						return json.RawMessage(`{}`), nil
					},
				})).To(Succeed())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("a.go")},
							{Name: agentloop.ToolCodeSignalReport, Args: json.RawMessage(`{"files":[{"path":"a.go"}]}`)},
							{Name: "third_tool", Args: json.RawMessage(`{}`)},
						},
					},
					agentloopharness.Response{Text: "should not reach"},
				)

				result, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, toolErr)).To(BeTrue())
				Expect(result.FinalText).NotTo(Equal("should not reach"))
				Expect(order).To(Equal([]string{
					agentloop.ToolSemanticsAnalyze,
					agentloop.ToolCodeSignalReport,
				}))
				Expect(thirdInvoked.Load()).To(BeFalse())
			})
		})

		When("a registered tool handler returns a non-budget error", func() {
			It("surfaces that error from Call and from Run without rewriting it as a budget or unknown-tool failure", func() {
				toolErr := fmt.Errorf("deterministic analyze failed: %w", errors.New("disk full"))
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return nil, toolErr
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, callErr := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("a.go"))
				Expect(callErr).To(HaveOccurred())
				Expect(errors.Is(callErr, toolErr)).To(BeTrue())
				Expect(errors.Is(callErr, agentloop.ErrBudgetExceeded)).To(BeFalse())
				Expect(errors.Is(callErr, agentloop.ErrUnknownTool)).To(BeFalse())
				Expect(errors.Is(callErr, agentloop.ErrInvalidArgs)).To(BeFalse())

				// Fresh loop so Call budget state does not interfere with Run.
				loop2, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return nil, toolErr
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("b.go")},
						},
					},
				)
				_, runErr := loop2.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(runErr).To(HaveOccurred())
				Expect(errors.Is(runErr, toolErr)).To(BeTrue())
				Expect(errors.Is(runErr, agentloop.ErrBudgetExceeded)).To(BeFalse())
			})
		})

		When("the model requests an unregistered tool name", func() {
			It("ends the run with ErrUnknownTool and does not execute any freeform action", func() {
				var coreCalls []agentloopharness.RecordedCall
				loop := newLoopWithRecordingCore(agentloop.Options{}, &coreCalls)

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						Text: "calling something shady",
						ToolCalls: []agentloopharness.ToolCall{
							{Name: "shell_exec", Args: json.RawMessage(`{"cmd":"rm -rf /"}`)},
						},
					},
				)

				_, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrUnknownTool)).To(BeTrue())
				Expect(coreCalls).To(BeEmpty())

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(1))
				Expect(recorded[0].Name).To(Equal("shell_exec"))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceModel))
				Expect(errors.Is(recorded[0].Err, agentloop.ErrUnknownTool)).To(BeTrue())
			})
		})

		When("the model returns tool args that fail the registered schema", func() {
			It("rejects the call with ErrInvalidArgs and does not invoke the handler", func() {
				var invoked atomic.Bool
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invoked.Store(true)
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: json.RawMessage(`{"path":"a.go"}`)},
						},
					},
				)

				_, err = loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrInvalidArgs)).To(BeTrue())
				Expect(invoked.Load()).To(BeFalse())
			})

			It("rejects codesignal_report files items missing path without invoking the handler", func() {
				var invoked atomic.Bool
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invoked.Store(true)
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport,
					json.RawMessage(`{"files":[{"status":"added"}],"diagnostics":[]}`))
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrInvalidArgs)).To(BeTrue())
				Expect(invoked.Load()).To(BeFalse())
			})
		})
	})

	Describe("core tool registration boundary", func() {
		When("a handler tries to Register over a core tool name", func() {
			It("rejects the registration so core tools stay on Options injection only", func() {
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = loop.Register(agentloop.ToolSpec{
					Name:    agentloop.ToolSemanticsAnalyze,
					Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) { return nil, nil },
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(agentloop.ToolSemanticsAnalyze))

				// Core handler still the one from Options, not replaced.
				var coreCalls []agentloopharness.RecordedCall
				loop2 := newLoopWithRecordingCore(agentloop.Options{}, &coreCalls)
				_, err = loop2.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("a.go"))
				Expect(err).NotTo(HaveOccurred())
				Expect(coreCalls).To(HaveLen(1))
			})
		})
	})

	Describe("default core tool wrappers", func() {
		When("Options leave SemanticsAnalyze and CodeSignalReport nil", func() {
			It("runs the real pkg/semantics and pkg/codesignal wrappers through the registry", func() {
				loop, err := agentloop.New(agentloop.Options{})
				Expect(err).NotTo(HaveOccurred())

				semOut, err := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze,
					json.RawMessage(`{"path":"hello.go","language":"go","content":"package hello\n"}`))
				Expect(err).NotTo(HaveOccurred())
				Expect(json.Valid(semOut)).To(BeTrue())
				Expect(semOut).To(ContainSubstring(`"path"`))

				csOut, err := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport,
					json.RawMessage(`{"files":[{"path":"hello.go"}],"baseline":true,"diagnostics":[]}`))
				Expect(err).NotTo(HaveOccurred())
				Expect(json.Valid(csOut)).To(BeTrue())
			})

			It("surfaces a partial semantics Result together with ErrSyntax for broken source", func() {
				// Task 8 must keep the partial JSON when AnalyzeBytes returns
				// (result, ErrSyntax); the loop must not drop the payload.
				loop, err := agentloop.New(agentloop.Options{})
				Expect(err).NotTo(HaveOccurred())

				out, callErr := loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze,
					json.RawMessage(`{"path":"broken.go","language":"go","content":"package p\nfunc ("}`))
				Expect(callErr).To(HaveOccurred())
				Expect(errors.Is(callErr, semantics.ErrSyntax)).To(BeTrue())
				Expect(out).NotTo(BeEmpty())
				Expect(json.Valid(out)).To(BeTrue())

				var partial map[string]any
				Expect(json.Unmarshal(out, &partial)).To(Succeed())
				Expect(partial["parse_status"]).To(Equal("syntax_errors"))
				Expect(partial["path"]).To(Equal("broken.go"))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(1))
				Expect(recorded[0].Result).NotTo(BeEmpty())
				Expect(errors.Is(recorded[0].Err, semantics.ErrSyntax)).To(BeTrue())
			})
		})
	})

	Describe("budget enforcement", func() {
		When("max_tool_calls is exhausted", func() {
			It("ends the run with ErrBudgetExceeded without invoking further tools", func() {
				var invokeCount atomic.Int32
				loop, err := agentloop.New(agentloop.Options{
					Budget: agentloop.Budget{
						MaxToolCalls:  1,
						MaxModelCalls: 20,
						MaxWallTime:   5 * time.Minute,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("a.go"))
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport, json.RawMessage(`{"files":[{"path":"a.go"}]}`))
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(invokeCount.Load()).To(Equal(int32(1)))
			})

			It("shares one max_tool_calls pool across handler-driven Call and model-selected Run on the same Loop", func() {
				// Task 8 drives the deterministic pass via Call then may Run
				// for model-selected evidence; both must debit the same budget.
				var invokeCount atomic.Int32
				loop, err := agentloop.New(agentloop.Options{
					Budget: agentloop.Budget{
						MaxToolCalls:  2,
						MaxModelCalls: 20,
						MaxWallTime:   5 * time.Minute,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("handler.go"))
				Expect(err).NotTo(HaveOccurred())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolCodeSignalReport, Args: json.RawMessage(`{"files":[{"path":"a.go"}]}`)},
						},
					},
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							// Third tool call overall must hit the shared budget.
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("too-many.go")},
						},
					},
					agentloopharness.Response{Text: "should not reach"},
				)

				result, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_tool_calls"))
				Expect(result.FinalText).NotTo(Equal("should not reach"))
				// Handler Call + first model tool succeeded; third attempt blocked.
				Expect(invokeCount.Load()).To(Equal(int32(2)))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(2))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceHandler))
				Expect(recorded[1].Source).To(Equal(agentloop.CallSourceModel))
			})
		})

		When("max_model_calls is exhausted", func() {
			It("completes exactly one model turn and one tool invoke, then ends with max_model_calls ErrBudgetExceeded", func() {
				var invokeCount atomic.Int32
				var generateCount atomic.Int32
				loop, err := agentloop.New(agentloop.Options{
					Budget: agentloop.Budget{
						MaxToolCalls:  50,
						MaxModelCalls: 1,
						MaxWallTime:   5 * time.Minute,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				// First model turn issues a tool call; second scripted response must not run.
				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{
						ToolCalls: []agentloopharness.ToolCall{
							{Name: agentloop.ToolSemanticsAnalyze, Args: validSemanticsArgs("a.go")},
						},
					},
					agentloopharness.Response{Text: "should not reach"},
				)
				gw := countingGateway{
					inner: harnessGateway{inner: scripted},
					n:     &generateCount,
				}

				result, err := loop.Run(context.Background(), gw, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_model_calls"))
				Expect(err.Error()).NotTo(ContainSubstring("max_tool_calls"))
				Expect(err.Error()).NotTo(ContainSubstring("max_wall_time"))
				// Off-by-one that rejects before the first Generate still yields
				// ErrBudgetExceeded; require the first turn actually ran.
				Expect(generateCount.Load()).To(Equal(int32(1)))
				Expect(invokeCount.Load()).To(Equal(int32(1)))
				Expect(result.FinalText).NotTo(Equal("should not reach"))
			})
		})

		When("max_wall_time is exhausted between calls under an injected clock", func() {
			It("ends the run with ErrBudgetExceeded without treating wall-time failure as a different path", func() {
				start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
				clock := acceptanceharness.NewFakeClock(start)

				var invokeCount atomic.Int32
				loop, err := agentloop.New(agentloop.Options{
					Clock: clock,
					Budget: agentloop.Budget{
						MaxToolCalls:  50,
						MaxModelCalls: 20,
						MaxWallTime:   5 * time.Minute,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{"ok":true}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{"ok":true}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("a.go"))
				Expect(err).NotTo(HaveOccurred())
				Expect(invokeCount.Load()).To(Equal(int32(1)))

				clock.Advance(5*time.Minute + time.Second)

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("b.go"))
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_wall_time"))
				Expect(invokeCount.Load()).To(Equal(int32(1)))
			})

			It("rejects a text-only Generate that overran max_wall_time rather than returning success", func() {
				start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
				clock := acceptanceharness.NewFakeClock(start)
				loop, err := agentloop.New(agentloop.Options{
					Clock: clock,
					Budget: agentloop.Budget{
						MaxToolCalls:  50,
						MaxModelCalls: 20,
						MaxWallTime:   5 * time.Minute,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				result, err := loop.Run(context.Background(), advanceClockGateway{
					clock: clock,
					by:    5*time.Minute + time.Second,
					text:  "late success",
				}, "prompt")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_wall_time"))
				Expect(result.FinalText).NotTo(Equal("late success"))
			})
		})

		When("max_wall_time elapses during in-flight work", func() {
			It("cancels an in-flight Generate and returns ErrBudgetExceeded", func() {
				loop, err := agentloop.New(agentloop.Options{
					Budget: agentloop.Budget{
						MaxToolCalls:  50,
						MaxModelCalls: 20,
						MaxWallTime:   40 * time.Millisecond,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				// Parent timeout is a safety net only; green path must hit loop wall budget first.
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				started := time.Now()
				_, err = loop.Run(ctx, blockUntilCancelGateway{}, "prompt")
				elapsed := time.Since(started)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_wall_time"))
				Expect(elapsed).To(BeNumerically("<", time.Second))
			})

			It("cancels an in-flight tool handler and returns ErrBudgetExceeded", func() {
				loop, err := agentloop.New(agentloop.Options{
					Budget: agentloop.Budget{
						MaxToolCalls:  50,
						MaxModelCalls: 20,
						MaxWallTime:   40 * time.Millisecond,
					},
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						<-ctx.Done()
						return nil, ctx.Err()
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				started := time.Now()
				_, err = loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, validSemanticsArgs("a.go"))
				elapsed := time.Since(started)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("max_wall_time"))
				Expect(elapsed).To(BeNumerically("<", time.Second))
			})
		})

		When("defaults are used without an explicit Budget", func() {
			It("applies v1 defaults of 50 tool calls, 20 model calls, and 5 minutes wall time", func() {
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				b := loop.Budget()
				Expect(b.MaxToolCalls).To(Equal(agentloop.DefaultMaxToolCalls))
				Expect(b.MaxModelCalls).To(Equal(agentloop.DefaultMaxModelCalls))
				Expect(b.MaxWallTime).To(Equal(agentloop.DefaultMaxWallTime))
				Expect(agentloop.DefaultMaxToolCalls).To(Equal(50))
				Expect(agentloop.DefaultMaxModelCalls).To(Equal(20))
				Expect(agentloop.DefaultMaxWallTime).To(Equal(5 * time.Minute))
			})
		})
	})

	Describe("registry as the only execution path", func() {
		When("the model emits freeform text with no tool calls", func() {
			It("returns the text without invoking any tool handler", func() {
				var invokeCount atomic.Int32
				loop, err := agentloop.New(agentloop.Options{
					SemanticsAnalyze: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{}`), nil
					},
					CodeSignalReport: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
						invokeCount.Add(1)
						return json.RawMessage(`{}`), nil
					},
				})
				Expect(err).NotTo(HaveOccurred())

				scripted := agentloopharness.NewScriptedGateway(
					agentloopharness.Response{Text: "just prose; not a tool call"},
				)

				result, err := loop.Run(context.Background(), harnessGateway{inner: scripted}, "prompt")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.FinalText).To(Equal("just prose; not a tool call"))
				Expect(invokeCount.Load()).To(Equal(int32(0)))
				Expect(loop.Calls()).To(BeEmpty())
			})
		})
	})
})
