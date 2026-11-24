package main

import (
	"strings"
	"unicode"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// modeAndTarget is one scan mode with one way of naming the policy.
type modeAndTarget struct {
	mode   scopePolicyMode
	target spellingTarget
}

// everyModeWith is every scan mode paired with each of targets.
func everyModeWith(targets []spellingTarget) []modeAndTarget {
	pairs := []modeAndTarget{}
	for _, mode := range scopePolicyModes {
		for _, target := range targets {
			pairs = append(pairs, modeAndTarget{mode, target})
		}
	}
	return pairs
}

// caseVariantsOf is count distinct names that differ from name only by case.
func caseVariantsOf(name string, count int) []string {
	variants := []string{}
	seen := map[string]bool{name: true}
	for mask := 1; len(variants) < count; mask++ {
		if variant := upperAt(name, mask); !seen[variant] {
			seen[variant] = true
			variants = append(variants, variant)
		}
	}
	return variants
}

// upperAt is name with the letters at the bits set in mask upper-cased.
func upperAt(name string, mask int) string {
	runes := []rune(name)
	for i := range runes {
		if mask&(1<<i) != 0 {
			runes[i] = unicode.ToUpper(runes[i])
		}
	}
	return string(runes)
}

// expectVariantsNamed checks that up to 16 variants are each named and one is chosen to keep, and that past 16 exactly
// 16 are named, the message says there are more, and none is chosen.
func expectVariantsNamed(stderr, directory string, variants []string, wantsAChoice bool) {
	GinkgoHelper()
	named := 0
	for _, variant := range variants {
		if strings.Contains(stderr, `"`+directory+variant+`"`) {
			named++
		}
	}
	if !wantsAChoice {
		Expect(stderr).To(ContainSubstring(", and more, and Coach reads"), "past 16 the message says there are more:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("To keep"), "past 16 no spelling is chosen:\n%s", stderr)
		Expect(named).To(Equal(16), "past 16 exactly 16 are named:\n%s", stderr)
		return
	}
	Expect(stderr).NotTo(ContainSubstring("and more"), "stderr:\n%s", stderr)
	Expect(stderr).To(ContainSubstring("To keep"), "up to 16 one is chosen to keep:\n%s", stderr)
	Expect(named).To(Equal(len(variants)), "every variant is named:\n%s", stderr)
}
