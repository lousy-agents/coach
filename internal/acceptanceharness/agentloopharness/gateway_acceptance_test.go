package agentloopharness_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness/agentloopharness"
)

var _ = Describe("ScriptedGateway", func() {
	Context("when Generate is called repeatedly", func() {
		It("returns the scripted responses in order, then ErrScriptExhausted instead of a zero value or panic", func() {
			first := agentloopharness.Response{Text: "first response"}
			second := agentloopharness.Response{
				Text: "second response",
				ToolCalls: []agentloopharness.ToolCall{
					{Name: "semantics_analyze", Args: json.RawMessage(`{"path":"a.go"}`)},
				},
			}

			gateway := agentloopharness.NewScriptedGateway(first, second)

			got1, err := gateway.Generate(context.Background(), "prompt one")
			Expect(err).NotTo(HaveOccurred())
			Expect(got1).To(Equal(first))

			got2, err := gateway.Generate(context.Background(), "prompt two")
			Expect(err).NotTo(HaveOccurred())
			Expect(got2).To(Equal(second))

			got3, err := gateway.Generate(context.Background(), "prompt three")
			Expect(err).To(MatchError(agentloopharness.ErrScriptExhausted))
			Expect(got3).To(Equal(agentloopharness.Response{}))
		})
	})
})
