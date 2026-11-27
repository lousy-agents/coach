package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// The CLI contract says what a missing security.toctou_check_then_act finding means, and which forms the rule does not
// match. These rows hold the contract's wording to the rule's stated boundary (#205): no finding is not evidence of
// safety, and the known misses are named. The CLI rows run the built binary on the cited shapes, so the section's
// claims about what matches are checked against the detector, not only against the text.
var _ = Describe("the security.toctou_check_then_act rule's documented boundary (#205)", func() {
	It("says that no finding means only that the syntactic rule did not match, and that the all-clear is not about this race", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("**No finding means only that this syntactic rule did not match.**"))
		Expect(text).To(ContainSubstring("It does not show that the code is free of time-of-check-to-time-of-use races."))
		Expect(text).To(ContainSubstring("The report's all-clear line, `No active CodeSignal findings`, describes the rules that ran. It says nothing more about this race."))
	})

	It("says each gating if or while yields one finding at its first matching act, and that the finding does not change the exit code", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("Each gating `if` or `while` that matches yields one medium-severity finding, located at the first matching act in its body. Later acts under the same gate get no finding of their own."))
		Expect(text).To(ContainSubstring("The finding does not change the exit code."))
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
		Entry("the Go walk callback acting on its own path variable", "An act in a walk callback on the callback's own path variable"),
		Entry("an act from another package", "An act from another package, such as `ioutil.ReadFile`."),
		Entry("the TypeScript do-while loop", "a `do` … `while` loop"),
		Entry("the TypeScript ternary", "A ternary, `existsSync(p) ? readFileSync(p) : null`, is not an `if` or `while`"),
		Entry("a renamed import", "A call under a renamed import is not matched"),
		Entry("the TypeScript negated condition", "a negation such as `!existsSync(p)`"),
		Entry("the TypeScript else-branch act", "An act that sits only in the `else` branch of an `if`."),
		Entry("the TypeScript async check", "`fs.promises.access`"),
		Entry("aliased and differently spelled paths", "`path.join(dir, name)` against `p`"),
		Entry("the absence of alias and interprocedural analysis", "There is no alias analysis, no interprocedural analysis, and no runtime proof."),
	)
})

// toctouFindings is the rule's signals in one baseline report, in the order the CLI reports them.
func toctouFindings(report *codesignal.Report) []codesignal.Signal {
	var found []codesignal.Signal
	for _, signal := range report.Signals {
		if signal.RuleID == "security.toctou_check_then_act" {
			found = append(found, signal)
		}
	}
	return found
}

// rowOf is the zero-based line of the first occurrence of needle in source, which is how a finding's StartRow counts.
func rowOf(source, needle string) uint {
	return uint(strings.Count(source[:strings.Index(source, needle)], "\n"))
}

var _ = Describe("the security.toctou_check_then_act rule as the built CLI reports it (#205)", func() {
	DescribeTable("reports the number of findings the section describes for each shape",
		func(file, source string, want int) {
			repo := newTempGitRepo()
			commitFile(repo, file, source)

			report, _ := runCoachCodesignalBaseline(repo)

			Expect(toctouFindings(report)).To(HaveLen(want))
		},
		Entry("a Go gate with one act reports one finding",
			"toctou.go", `package main

import "os"

func f(p string) error {
	if _, err := os.Stat(p); err == nil {
		_, err := os.ReadFile(p)
		return err
	}
	return nil
}
`, 1),
		Entry("a Go gate whose act names another path reports none",
			"toctou.go", `package main

import "os"

func f(p, q string) error {
	if _, err := os.Stat(p); err == nil {
		return os.Remove(q)
	}
	return nil
}
`, 0),
		Entry("a Go err != nil early return reports none",
			"toctou.go", `package main

import "os"

func f(p string) error {
	if _, err := os.Stat(p); err != nil {
		return err
	}
	_, err := os.Open(p)
	return err
}
`, 0),
		Entry("a Go walk callback that acts on the gated path reports one finding",
			"toctou.go", `package main

import (
	"os"
	"path/filepath"
)

func f(p string) error {
	if _, err := os.Stat(p); err == nil {
		return filepath.Walk(p, func(q string, info os.FileInfo, err error) error {
			return os.Remove(p)
		})
	}
	return nil
}
`, 1),
		Entry("a Go walk callback that acts on its own path variable reports none",
			"toctou.go", `package main

import (
	"os"
	"path/filepath"
)

func f(p string) error {
	if _, err := os.Stat(p); err == nil {
		return filepath.Walk(p, func(q string, info os.FileInfo, err error) error {
			return os.Remove(q)
		})
	}
	return nil
}
`, 0),
		Entry("a TypeScript existsSync guard over readFileSync reports one finding",
			"toctou.ts", `import { existsSync, readFileSync } from "node:fs";

export function read(p: string) {
  if (existsSync(p)) {
    return readFileSync(p);
  }
  return undefined;
}
`, 1),
		Entry("a Go call under a renamed os import reports none",
			"toctou.go", `package main

import o "os"

func f(p string) error {
	if _, err := o.Stat(p); err == nil {
		return o.Remove(p)
	}
	return nil
}
`, 0),
		Entry("a TypeScript call under a renamed existsSync import reports none",
			"toctou.ts", `import { existsSync as ex, readFileSync } from "node:fs";

export function read(p: string) {
  if (ex(p)) {
    return readFileSync(p);
  }
  return undefined;
}
`, 0),
		Entry("a TypeScript ternary reports none",
			"toctou.ts", `import { existsSync, readFileSync } from "node:fs";

export function read(p: string) {
  return existsSync(p) ? readFileSync(p) : null;
}
`, 0),
		Entry("a negated TypeScript guard reports none",
			"toctou.ts", `import { existsSync, readFileSync } from "node:fs";

export function read(p: string) {
  if (!existsSync(p)) {
    return readFileSync(p);
  }
  return undefined;
}
`, 0),
	)

	It("reports one finding for two acts under one gate, located at the first act", func() {
		source := `package main

import "os"

func f(p string) error {
	if _, err := os.Stat(p); err == nil {
		os.Remove(p)
		_, err := os.ReadFile(p)
		return err
	}
	return nil
}
`
		repo := newTempGitRepo()
		commitFile(repo, "toctou.go", source)

		report, _ := runCoachCodesignalBaseline(repo)

		found := toctouFindings(report)
		Expect(found).To(HaveLen(1))
		Expect(found[0].Location.StartRow).To(Equal(rowOf(source, "os.Remove(p)")))
	})
})
