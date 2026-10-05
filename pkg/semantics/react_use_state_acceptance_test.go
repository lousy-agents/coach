package semantics_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): useState bindings", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	When("a destructuring useState binding has an elided (hole) first element, as in const [, setC] = useState(2)", func() {
		It("shall keep the setter in the setter slot rather than shifting it into the binding slot", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [, setC] = useState(2);
  const [x, setX] = useState(1);
  return <div>{x}</div>;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.UseState).To(HaveLen(2))
			if len(rec.UseState) == 2 {
				Expect(rec.UseState[0].Binding).To(Equal(""))
				Expect(rec.UseState[0].Setter).To(Equal("setC"))
				Expect(rec.UseState[1].Binding).To(Equal("x"))
				Expect(rec.UseState[1].Setter).To(Equal("setX"))
			}
		})
	})
})
