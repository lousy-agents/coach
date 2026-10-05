package projectmodel_test

import (
	"reflect"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func expectReachabilityFactIsFactsOnly() {
	allowed := map[string]bool{
		"ID":               true,
		"Kind":             true,
		"Confidence":       true,
		"Source":           true,
		"Sink":             true,
		"Path":             true,
		"AlgorithmVersion": true,
	}
	forbiddenSubstrings := []string{"severity", "lifecycle", "changed", "active", "finding", "status"}

	factType := reflect.TypeOf(projectmodel.ReachabilityFact{})
	seen := map[string]bool{}
	for i := 0; i < factType.NumField(); i++ {
		name := factType.Field(i).Name
		seen[name] = true
		Expect(allowed).To(HaveKey(name), "ReachabilityFact gained an unexpected field %q not on the reviewed allowlist", name)
		lower := strings.ToLower(name)
		for _, bad := range forbiddenSubstrings {
			Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityFact field %q looks severity/lifecycle/active-finding-shaped", name)
		}
	}
	for name := range allowed {
		Expect(seen).To(HaveKey(name), "expected allowlisted field %q to still exist on ReachabilityFact", name)
	}

	stepType := reflect.TypeOf(projectmodel.ReachabilityStep{})
	for i := 0; i < stepType.NumField(); i++ {
		name := stepType.Field(i).Name
		lower := strings.ToLower(name)
		for _, bad := range forbiddenSubstrings {
			Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityStep field %q looks severity/lifecycle/active-finding-shaped", name)
		}
	}
}

func expectReachabilityResultIsFactsOnly() {
	allowedResultFields := map[string]bool{
		"Facts":     true,
		"Sources":   true,
		"Algorithm": true,
		"Coverage":  true,
	}
	forbiddenSubstrings := []string{"severity", "lifecycle", "changed", "active", "finding", "status"}

	resultType := reflect.TypeOf(projectmodel.ReachabilityResult{})
	seen := map[string]bool{}
	for i := 0; i < resultType.NumField(); i++ {
		name := resultType.Field(i).Name
		seen[name] = true
		Expect(allowedResultFields).To(HaveKey(name), "ReachabilityResult gained an unexpected field %q not on the reviewed allowlist", name)
		lower := strings.ToLower(name)
		for _, bad := range forbiddenSubstrings {
			Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityResult field %q looks severity/lifecycle/active-finding-shaped", name)
		}
	}
	for name := range allowedResultFields {
		Expect(seen).To(HaveKey(name), "expected allowlisted field %q to still exist on ReachabilityResult", name)
	}
}
