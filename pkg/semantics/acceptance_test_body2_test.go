package semantics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_acceptanceTest_detectsPointerReturningFunctionsAndMethodsAC36_406(analyze func(source string) *semantics.Result) {
	result := analyze(`package main

func NewThing() *int { return nil }

type T struct{}

func (t T) Get() *int { return nil }

func Value() int { return 0 }
`)

	names := map[string]bool{}
	for _, f := range result.Findings {
		if f.Kind == "pointer_return" {
			names[f.Name] = true
		}
	}
	Expect(names).To(HaveKey("NewThing"))
	Expect(names).To(HaveKey("Get"))
	Expect(names).NotTo(HaveKey("Value"))
}
