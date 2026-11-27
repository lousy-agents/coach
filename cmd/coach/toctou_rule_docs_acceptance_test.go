package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The CLI contract says what a missing security.toctou_check_then_act finding means, and which forms the rule does not
// match. These rows hold the contract's wording to the rule's stated boundary (#205): no finding is not evidence of
// safety, and the known misses are named.
var _ = Describe("the security.toctou_check_then_act rule's documented boundary (#205)", func() {
	It("says that no finding means only that the syntactic rule did not match, and that the all-clear is not about this race", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("**No finding means only that this syntactic rule did not match.**"))
		Expect(text).To(ContainSubstring("It does not show that the code is free of time-of-check-to-time-of-use races."))
		Expect(text).To(ContainSubstring("The report's all-clear line, `No active CodeSignal findings`, describes the rules that ran."))
	})

	It("says the rule matches names and not bindings, so a same-named method on another object matches", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("The rule matches names, not bindings."))
		Expect(text).To(ContainSubstring("`cache.existsSync(p)` followed by `cache.readFileSync(p)` matches even when `cache` is not `fs`"))
	})

	DescribeTable("names each known miss the rule has",
		func(miss string) {
			Expect(documentText("docs/cli-codesignal.md")).To(ContainSubstring(miss))
		},
		Entry("the Go err != nil early return", "an `err != nil` early return"),
		Entry("the Go errors.Is sentinel gate", "`errors.Is(err, fs.ErrNotExist)`"),
		Entry("the Go os.IsNotExist gate", "`os.IsNotExist(err)`"),
		Entry("a Stat in a statement before the if", "A `Stat` or `Lstat` in a statement before the `if`"),
		Entry("the Go create acts that are not matched", "`os.Create`, `os.Mkdir`, `os.MkdirAll`, `os.WriteFile`, or `os.Rename`"),
		Entry("the TypeScript negated condition", "a negation such as `!existsSync(p)`"),
		Entry("the TypeScript else-branch act", "An act that sits only in the `else` branch of an `if`."),
		Entry("the TypeScript async check", "`fs.promises.access`"),
		Entry("aliased and differently spelled paths", "`path.join(dir, name)` against `p`"),
		Entry("the absence of alias and interprocedural analysis", "There is no alias analysis, no interprocedural analysis, and no runtime proof."),
	)
})
