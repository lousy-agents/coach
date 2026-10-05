package agentloopharness_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness/agentloopharness"
)

var _ = Describe("RecordingToolRegistry", func() {
	Context("when a handler-driven call and a model-selected call are both made", func() {
		It("records both, distinguishable by CallSource, in call order", func() {
			registry := &agentloopharness.RecordingToolRegistry{}
			registry.Register("semantics_analyze", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"status":"ok"}`), nil
			})

			_, err := registry.Call(context.Background(), agentloopharness.CallSourceHandler, "semantics_analyze", json.RawMessage(`{"path":"a.go"}`))
			Expect(err).NotTo(HaveOccurred())

			_, err = registry.Call(context.Background(), agentloopharness.CallSourceModel, "semantics_analyze", json.RawMessage(`{"path":"b.go"}`))
			Expect(err).NotTo(HaveOccurred())

			calls := registry.Calls()
			Expect(calls).To(HaveLen(2))
			Expect(calls[0].Source).To(Equal(agentloopharness.CallSourceHandler))
			Expect(calls[0].Name).To(Equal("semantics_analyze"))
			Expect(calls[1].Source).To(Equal(agentloopharness.CallSourceModel))
			Expect(calls[1].Name).To(Equal("semantics_analyze"))
		})
	})

	Context("when Call targets an unregistered tool name", func() {
		It("returns ErrUnknownTool and still appears in Calls(), not silently dropped", func() {
			registry := &agentloopharness.RecordingToolRegistry{}

			_, err := registry.Call(context.Background(), agentloopharness.CallSourceModel, "nonexistent_tool", json.RawMessage(`{}`))
			Expect(err).To(MatchError(agentloopharness.ErrUnknownTool))

			calls := registry.Calls()
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Name).To(Equal("nonexistent_tool"))
			Expect(calls[0].Source).To(Equal(agentloopharness.CallSourceModel))
			Expect(calls[0].Err).To(MatchError(agentloopharness.ErrUnknownTool))
		})
	})

	Context("when a registered handler itself returns an error", func() {
		It("still records the call with that error as the outcome", func() {
			registry := &agentloopharness.RecordingToolRegistry{}
			registry.Register("failing_tool", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
				return nil, agentloopharness.ErrUnknownTool
			})

			_, err := registry.Call(context.Background(), agentloopharness.CallSourceHandler, "failing_tool", json.RawMessage(`{}`))
			Expect(err).To(HaveOccurred())

			calls := registry.Calls()
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Err).To(HaveOccurred())
		})
	})

	Context("when Calls is called after recording", func() {
		It("returns a defensive copy that mutating does not affect the registry's internal state", func() {
			registry := &agentloopharness.RecordingToolRegistry{}
			registry.Register("tool_a", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			})
			_, err := registry.Call(context.Background(), agentloopharness.CallSourceHandler, "tool_a", json.RawMessage(`{}`))
			Expect(err).NotTo(HaveOccurred())

			calls := registry.Calls()
			calls[0].Name = "mutated"

			callsAgain := registry.Calls()
			Expect(callsAgain[0].Name).To(Equal("tool_a"))
		})
	})

	Context("when the args/result byte slices are mutated after the fact", func() {
		It("does not change what Calls() later observes, whether mutated via the original buffers or a prior Calls() result", func() {
			registry := &agentloopharness.RecordingToolRegistry{}
			registry.Register("tool_b", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"result":"orig"}`), nil
			})

			args := json.RawMessage(`{"arg":"orig"}`)
			_, err := registry.Call(context.Background(), agentloopharness.CallSourceHandler, "tool_b", args)
			Expect(err).NotTo(HaveOccurred())

			args[2] = 'X'

			calls := registry.Calls()
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Args).To(MatchJSON(`{"arg":"orig"}`))
			Expect(calls[0].Result).To(MatchJSON(`{"result":"orig"}`))

			calls[0].Args[2] = 'Y'
			calls[0].Result[2] = 'Y'

			callsAgain := registry.Calls()
			Expect(callsAgain[0].Args).To(MatchJSON(`{"arg":"orig"}`))
			Expect(callsAgain[0].Result).To(MatchJSON(`{"result":"orig"}`))
		})
	})

	Context("when multiple goroutines call Call concurrently", func() {
		It("is safe under -race and every call is recorded", func() {
			body_registryAcceptanceTest_isSafeUnderRaceAndEveryCallIsRecorded_113()
		})
	})
})
