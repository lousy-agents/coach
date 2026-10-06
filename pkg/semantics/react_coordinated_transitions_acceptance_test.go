package semantics_test

import (
	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): coordinated transitions", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	When("a nested PascalCase inner component calls two useState setters from the outer scope", func() {
		It("shall attribute no coordinated transition to the outer component", func() {
			const src = `"use client";

import { useState } from "react";

export function Outer() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  function InnerPanel() {
    setA(1);
    setB(2);
    return <section />;
  }
  return (
    <div>
      <InnerPanel />
    </div>
  );
}
`
			result := analyzeTSX(analyzer, "Outer.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Outer")
			Expect(ok).To(BeTrue(), "expected a react_components record named Outer, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(BeEmpty(), "InnerPanel's setter calls must not attribute a coordinated transition to Outer, got %+v", rec.CoordinatedTransitions)
		})
	})

	When("a two-setter-calling arrow function is the second (not first) argument of a useEffect call", func() {
		It("shall classify the transition as callback, not effect", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  useEffect(null, () => {
    setA(1);
    setB(2);
  });
  return <div>{a}</div>;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1), "expected exactly one coordinated transition")
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("callback"), "an arrow function in the second argument position of useEffect must not be classified as an effect")
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("<anonymous>"), "anonymous callbacks must use the <anonymous> name sentinel, not empty string")
			}
		})
	})

	When("a non-handler-named local callback updates two state bindings", func() {
		It("shall record the assigned binding as the transition name with kind callback", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  const run = () => {
    setA(1);
    setB(2);
  };
  return <button type="button" onClick={run} />;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("callback"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("run"), "assigned non-on*/handle* callbacks must keep their binding name")
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
			}
		})
	})

	When("an inline JSX onClick arrow updates two state bindings", func() {
		It("shall record a coordinated transition with kind handler and name onClick", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  return (
    <button
      type="button"
      onClick={() => {
        setA(1);
        setB(2);
      }}
    />
  );
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("handler"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("onClick"))
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
				assertNonEmptyLocation(rec.CoordinatedTransitions[0].Location, "onClick handler")
			}
		})
	})

	When("a local handle* binding updates two state bindings", func() {
		It("shall record a coordinated transition with kind handler and the binding name", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  const handleReset = () => {
    setA(1);
    setB(2);
  };
  return <button type="button" onClick={handleReset} />;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("handler"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("handleReset"))
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
			}
		})
	})
})
