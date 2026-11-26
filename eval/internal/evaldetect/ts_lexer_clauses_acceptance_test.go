package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

// regexKeywordRows puts a regular expression holding a quote after each
// keyword that lets a slash start one. Read as a division, the quote would
// open a string that swallows the import after it.
func regexKeywordRows() []TableEntry {
	var rows []TableEntry
	for _, keyword := range []string{"return", "typeof", "instanceof", "in", "of", "new", "delete", "void", "throw", "case", "do", "else", "yield", "await"} {
		rows = append(rows, Entry("a regular expression after "+keyword, keyword+" /\"/; import \"../app/a\";\n", 1))
	}
	return rows
}

func domainViolations(domainSource string) ([]evaldetect.Finding, error) {
	return tsViolations(tsTree(map[string]string{"src/domain/d.ts": domainSource}))
}

var _ = Describe("the TypeScript lexer's clauses", func() {
	DescribeTable("an import the lexer must see, at its own line",
		func(domainSource string, line int) {
			findings, err := domainViolations(domainSource)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Line", line)))
		},
		Entry("a single-quoted specifier", "import { A } from '../app/a';\n", 1),
		Entry("a tab between import and its specifier", "import\t\"../app/a\";\n", 1),
		Entry("a carriage return between import and its specifier", "import\r\"../app/a\";\n", 1),
		Entry("after a string holding an escaped quote", "const s = \"a\\\"b\"; import \"../app/a\";\n", 1),
		Entry("after a template holding an escaped backtick", "const t = `a\\`b`; import \"../app/a\";\n", 1),
		Entry("on the line after a string that a newline ends", "const s = \"open\nimport \"../app/a\";\n", 2),
		Entry("on the line after a string continued by a backslash", "const s = \"a\\\nb\";\nimport(\"../app/a\");\n", 3),
		Entry("after a regular expression holding an escaped slash", "const r = /\\/\"/; import \"../app/a\";\n", 1),
		Entry("after a regular expression whose class holds a slash", "const r = /[/\"]/; import \"../app/a\";\n", 1),
		Entry("on the line after a regular expression that a newline ends", "const r = /abc\nimport \"../app/a\";\n", 2),
		Entry("after a division that follows )", "const D = f(1) / 2; import { A } from \"../app/a\";\n", 1),
		Entry("after a division that follows ]", "const D = xs[0] / 2; import { A } from \"../app/a\";\n", 1),
		Entry("after a division that follows }", "const D = {} / 2; import { A } from \"../app/a\";\n", 1),
		Entry("after a division that follows a postfix ++", "let i = 0; const D = i++ / 2; const b = import(\"../app/a\");\n", 1),
		Entry("after a division that follows a keyword used as a property", "const a = { of: 4 }; const D = a.of / 2; const b = import(\"../app/a\");\n", 1),
		Entry("after a regular expression that starts the file", "/\"/.test(\"x\"); import \"../app/a\";\n", 1),
		Entry("as export * from", "export * from \"../app/a\";\n", 1),
		Entry("as export type {} from", "export type { A } from \"../app/a\";\n", 1),
		Entry("as import() with an options argument", "const m = import(\"../app/a\", { with: { type: \"json\" } });\n", 1),
		Entry("once, when an export list without from precedes an import", "export { D }\nimport { A } from \"../app/a\";\nconst D = 1;\n", 2),
		Entry("once, when an export list without from precedes a re-export", "export { D }\nexport { A } from \"../app/a\";\nconst D = 1;\n", 2),
		regexKeywordRows(),
	)

	DescribeTable("source that names a specifier without importing it reports nothing",
		func(domainSource string) {
			findings, err := domainViolations(domainSource)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		},
		Entry("an unterminated block comment", "/* never closed\nimport \"../app/a\";\n"),
		Entry("a method named import", "declare const foo: any; foo.import(\"../app/a\");\n"),
		Entry("a function named require", "function require(id: string) { return id; }\nexport const D = 1;\n"),
		Entry("a tagged template named from after a statement end", "export { D };\nfrom`../app/a`;\nconst D = 1;\n"),
		Entry("a tagged template named from after an assignment", "export { D }\nconst x = from`../app/a`;\nconst D = 1;\n"),
	)

	DescribeTable("source the lexer cannot read exactly is refused by name",
		func(files map[string]string, reason string) {
			_, err := tsViolations(tsTree(files))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring(reason)))
		},
		Entry("require() of a literal plus more", map[string]string{
			"src/domain/d.ts": "declare const require: (id: string) => unknown;\nconst name = \"a\";\nexport const app = require(\"../app/\" + name);\n",
		}, "non-literal require()"),
		Entry("a .tsx source, whose JSX text the lexer does not read", map[string]string{
			"src/domain/d.tsx": "export const D = () => <p>Don't</p>;\n",
		}, ".tsx"),
	)
})
