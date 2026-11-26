package evalpatterns_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// profileOfFiles extracts a profile from a module holding only files.
func profileOfFiles(files map[string]string) map[string]any {
	GinkgoHelper()
	root := filepath.Join(GinkgoT().TempDir(), "one")
	writeFile(filepath.Join(root, "go.mod"), "module example.invalid/one\n\ngo 1.26.0\n")
	for name, content := range files {
		writeFile(filepath.Join(root, filepath.FromSlash(name)), content)
	}
	return decodeProfile(extract(root))
}

func profileOfSource(source string) map[string]any {
	GinkgoHelper()
	return profileOfFiles(map[string]string{"p/p.go": source})
}

func measure(profile map[string]any, group, name string) any {
	return profile[group].(map[string]any)[name]
}

var _ = Describe("the measure of one function (AC-2.3, AC-2.5, AC-2.6)", func() {
	DescribeTable("classifies an exported function's entry kind by its signature",
		func(source string, kinds map[string]any) {
			Expect(measure(profileOfSource(source), "dimensions", "entry")).To(Equal(kinds))
		},
		Entry("a handler through an aliased net/http import",
			"package p\n\nimport h \"net/http\"\n\nfunc H(w h.ResponseWriter, r *h.Request) {}\n",
			map[string]any{"http_handler": 1.0, "method": 0.0, "function": 0.0}),
		Entry("a handler-shaped method not named ServeHTTP, which is a method",
			"package p\n\nimport \"net/http\"\n\ntype T struct{}\n\nfunc (T) Other(w http.ResponseWriter, r *http.Request) {}\n",
			map[string]any{"http_handler": 0.0, "method": 1.0, "function": 0.0}),
		Entry("a request taken by value, which is a function",
			"package p\n\nimport \"net/http\"\n\nfunc F(w http.ResponseWriter, r http.Request) {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 1.0}),
		Entry("a handler shape from another package named http, which is a function",
			"package p\n\nimport http \"example.invalid/fake\"\n\nfunc F(w http.ResponseWriter, r *http.Request) {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 1.0}),
		Entry("an exported method on an unexported type, which no caller outside can name",
			"package p\n\ntype t struct{}\n\nfunc (t) Do() {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 0.0}),
		Entry("an exported method on an exported generic type",
			"package p\n\ntype Box[T any] struct{ v T }\n\nfunc (b *Box[T]) Get() T { return b.v }\n",
			map[string]any{"http_handler": 0.0, "method": 1.0, "function": 0.0}),
		Entry("an unexported function, which is no entry",
			"package p\n\nfunc helper() {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 0.0}),
	)

	DescribeTable("classifies a New… function's constructor shape",
		func(source string, shape map[string]any) {
			Expect(measure(profileOfSource(source), "shapes", "constructor")).To(Equal(shape))
		},
		Entry("a bare New returning a literal",
			"package p\n\ntype T struct{}\n\nfunc New() *T { return &T{} }\n",
			map[string]any{"assigns_only": 1.0, "does_work": 0.0}),
		Entry("allocation by make before the literal",
			"package p\n\ntype T struct{ m map[string]int }\n\nfunc NewT() T {\n\tm := make(map[string]int)\n\treturn T{m: m}\n}\n",
			map[string]any{"assigns_only": 1.0, "does_work": 0.0}),
		Entry("a call inside the returned literal",
			"package p\n\ntype T struct{ n int }\n\nfunc compute() int { return 1 }\n\nfunc NewT() T { return T{n: compute()} }\n",
			map[string]any{"assigns_only": 0.0, "does_work": 1.0}),
		Entry("a branch before the return",
			"package p\n\ntype T struct{}\n\nfunc NewT(ok bool) *T {\n\tif !ok {\n\t\treturn nil\n\t}\n\treturn &T{}\n}\n",
			map[string]any{"assigns_only": 0.0, "does_work": 1.0}),
		Entry("Newer, which is not a constructor name",
			"package p\n\nfunc Newer() int { return 1 }\n",
			map[string]any{"assigns_only": 0.0, "does_work": 0.0}),
		Entry("a method named NewChild, which is not a constructor",
			"package p\n\ntype T struct{}\n\nfunc (T) NewChild() T { return T{} }\n",
			map[string]any{"assigns_only": 0.0, "does_work": 0.0}),
		Entry("a New… function with no result, which is not a constructor",
			"package p\n\nfunc NewNothing() {}\n",
			map[string]any{"assigns_only": 0.0, "does_work": 0.0}),
	)

	DescribeTable("measures a function's nesting",
		func(body string, nesting map[string]any) {
			source := "package p\n\nfunc F(a, b, c bool, v any, ch chan int) {\n" + body + "\n}\n"
			Expect(measure(profileOfSource(source), "shapes", "nesting")).To(Equal(nesting))
		},
		Entry("an if inside an else block, one deeper", "if a {\n} else {\n\tif b {\n\t}\n}", map[string]any{"2": 1.0}),
		Entry("an if inside an else-if, one deeper than the chain", "if a {\n} else if b {\n\tif c {\n\t}\n}", map[string]any{"2": 1.0}),
		Entry("an else-if chain, at its if's depth", "if a {\n} else if b {\n} else if c {\n}", map[string]any{"1": 1.0}),
		Entry("a type switch in a select in a switch", "switch {\ncase a:\n\tselect {\n\tcase <-ch:\n\t\tswitch v.(type) {\n\t\t}\n\t}\n}", map[string]any{"3": 1.0}),
	)

	When("code sits under a directory whose name starts with . or _, which the go tool ignores", func() {
		It("is not measured", func() {
			profile := profileOfFiles(map[string]string{
				"p/p.go":        "package p\n\nfunc F() {}\n",
				".hidden/h.go":  "package h\n\nfunc G() { if true {} }\n",
				"_skipped/s.go": "package s\n\nfunc G() { if true {} }\n",
			})

			Expect(measure(profile, "shapes", "nesting")).To(Equal(map[string]any{"0": 1.0}))
		})
	})
})
