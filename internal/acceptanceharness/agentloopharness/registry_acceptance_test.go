package agentloopharness_test

import (
	"context"
	"encoding/json"
	"sync"

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

	Context("when multiple goroutines call Call concurrently", func() {
		It("is safe under -race and every call is recorded", func() {
			registry := &agentloopharness.RecordingToolRegistry{}
			registry.Register("concurrent_tool", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			})

			const workers = 8
			const iterationsPerWorker = 50
			const total = workers * iterationsPerWorker

			var wg sync.WaitGroup
			wg.Add(workers)
			for i := 0; i < workers; i++ {
				go func(worker int) {
					defer wg.Done()
					for j := 0; j < iterationsPerWorker; j++ {
						_, _ = registry.Call(context.Background(), agentloopharness.CallSourceModel, "concurrent_tool", json.RawMessage(`{}`))
					}
				}(i)
			}
			wg.Wait()

			Expect(registry.Calls()).To(HaveLen(total))
		})
	})
})
