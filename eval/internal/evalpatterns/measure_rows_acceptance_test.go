package evalpatterns_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	assignsOnly   = map[string]any{"assigns_only": 1.0, "does_work": 0.0}
	doesWork      = map[string]any{"assigns_only": 0.0, "does_work": 1.0}
	noConstructor = map[string]any{"assigns_only": 0.0, "does_work": 0.0}
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
		Entry("a third parameter grouped with the request, which is a function",
			"package p\n\nimport \"net/http\"\n\nfunc F(w http.ResponseWriter, r, extra *http.Request) {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 1.0}),
		Entry("an exported method on a generic type with two type parameters",
			"package p\n\ntype Pair[K comparable, V any] struct{ v V }\n\nfunc (p Pair[K, V]) Value() V { return p.v }\n",
			map[string]any{"http_handler": 0.0, "method": 1.0, "function": 0.0}),
		Entry("a handler with unnamed parameters",
			"package p\n\nimport \"net/http\"\n\nfunc H(http.ResponseWriter, *http.Request) {}\n",
			map[string]any{"http_handler": 1.0, "method": 0.0, "function": 0.0}),
		Entry("an unexported function, which is no entry",
			"package p\n\nfunc helper() {}\n",
			map[string]any{"http_handler": 0.0, "method": 0.0, "function": 0.0}),
	)

	DescribeTable("classifies a New… function's constructor shape",
		func(source string, shape map[string]any) {
			Expect(measure(profileOfSource(source), "shapes", "constructor")).To(Equal(shape))
		},
		Entry("a bare New returning a literal",
			"package p\n\ntype T struct{}\n\nfunc New() *T { return &T{} }\n", assignsOnly),
		Entry("allocation by make before the literal",
			"package p\n\ntype T struct{ m map[string]int }\n\nfunc NewT() T {\n\tm := make(map[string]int)\n\treturn T{m: m}\n}\n", assignsOnly),
		Entry("a call to a pure helper, which is not real work",
			"package p\n\ntype T struct{ n int }\n\nfunc compute() int { return 1 }\n\nfunc NewT() T { return T{n: compute()} }\n", assignsOnly),
		Entry("errors.New and another constructor, which are not real work",
			"package p\n\nimport \"errors\"\n\ntype T struct{ err error; u *U }\n\ntype U struct{}\n\nfunc NewU() *U { return &U{} }\n\nfunc NewT() T { return T{err: errors.New(\"x\"), u: NewU()} }\n",
			map[string]any{"assigns_only": 2.0, "does_work": 0.0}),
		Entry("a branch before the return, which is not real work",
			"package p\n\ntype T struct{}\n\nfunc NewT(ok bool) *T {\n\tif !ok {\n\t\treturn nil\n\t}\n\treturn &T{}\n}\n", assignsOnly),
		Entry("an outward call through an aliased import",
			"package p\n\nimport o \"os\"\n\ntype T struct{ v string }\n\nfunc NewT() T { return T{v: o.Getenv(\"V\")} }\n", doesWork),
		Entry("os.IsNotExist, which reads no file and is not on the outward list",
			"package p\n\nimport \"os\"\n\ntype T struct{ missing bool }\n\nfunc NewT(err error) T { return T{missing: os.IsNotExist(err)} }\n", assignsOnly),
		Entry("http.Header, which shares the leading characters of Head but is a conversion, not a request",
			"package p\n\nimport \"net/http\"\n\ntype T struct{ h http.Header }\n\nfunc NewT() T { return T{h: http.Header(nil)} }\n", assignsOnly),
		Entry("http.NewRequest, which builds a request without sending it",
			"package p\n\nimport \"net/http\"\n\ntype T struct{ err error }\n\nfunc NewT() T { _, err := http.NewRequest(\"GET\", \"u\", nil); return T{err: err} }\n", assignsOnly),
		Entry("Do on another type built in place, which is not an http.Client",
			"package p\n\nimport \"net/http\"\n\ntype fake struct{}\n\nfunc (fake) Do(*http.Request) error { return nil }\n\ntype T struct{ err error }\n\nfunc NewT() T { return T{err: (&fake{}).Do(nil)} }\n", assignsOnly),
		Entry("another method on an http.Client built in place, which sends nothing",
			"package p\n\nimport \"net/http\"\n\ntype T struct{}\n\nfunc NewT() *T { (&http.Client{}).CloseIdleConnections(); return &T{} }\n", assignsOnly),
		Entry("a method named Getenv on a value, not the os package",
			"package p\n\ntype env struct{}\n\nfunc (env) Getenv(string) string { return \"\" }\n\ntype T struct{ v string }\n\nfunc NewT(os env) T { return T{v: os.Getenv(\"V\")} }\n", assignsOnly),
		Entry("Newer, which is not a constructor name",
			"package p\n\nimport \"os\"\n\nfunc Newer() string { return os.Getenv(\"V\") }\n", noConstructor),
		Entry("a method named NewChild, which is not a constructor",
			"package p\n\nimport \"os\"\n\ntype T struct{}\n\nfunc (T) NewChild() string { return os.Getenv(\"V\") }\n", noConstructor),
		Entry("a New… function with no result, which is not a constructor",
			"package p\n\nimport \"os\"\n\nfunc NewNothing() { os.Getenv(\"V\") }\n", noConstructor),
	)

	// One row per entry of the closed outward-API list (the chief's rulings on
	// #872 and #887: all filesystem, environment, process, network and SQL
	// I/O), and for each prefix a row with a name other than its base.
	DescribeTable("counts a constructor that calls an outward API as doing work",
		func(path, call string) {
			source := "package p\n\nimport \"" + path + "\"\n\ntype T struct{}\n\nfunc NewT() *T {\n\t_ = " + call + "\n\treturn &T{}\n}\n"
			Expect(measure(profileOfSource(source), "shapes", "constructor")).To(Equal(doesWork))
		},
		Entry("os.Getenv", "os", `os.Getenv("V")`),
		Entry("os.LookupEnv", "os", `func() string { v, _ := os.LookupEnv("V"); return v }()`),
		Entry("os.Environ", "os", `os.Environ()`),
		Entry("os.Open, the base of Open*", "os", `func() error { _, err := os.Open("f"); return err }()`),
		Entry("os.OpenFile, an Open* name", "os", `func() error { _, err := os.OpenFile("f", 0, 0); return err }()`),
		Entry("os.Create, the base of Create*", "os", `func() error { _, err := os.Create("f"); return err }()`),
		Entry("os.CreateTemp, a Create* name", "os", `func() error { _, err := os.CreateTemp("", "x"); return err }()`),
		Entry("os.ReadFile", "os", `func() error { _, err := os.ReadFile("f"); return err }()`),
		Entry("os.WriteFile", "os", `os.WriteFile("f", nil, 0)`),
		Entry("os.ReadDir", "os", `func() error { _, err := os.ReadDir("d"); return err }()`),
		Entry("os.Stat", "os", `func() error { _, err := os.Stat("f"); return err }()`),
		Entry("os.Lstat", "os", `func() error { _, err := os.Lstat("f"); return err }()`),
		Entry("os.Mkdir, the base of Mkdir*", "os", `os.Mkdir("d", 0)`),
		Entry("os.MkdirAll, a Mkdir* name", "os", `os.MkdirAll("d", 0)`),
		Entry("os.Remove, the base of Remove*", "os", `os.Remove("f")`),
		Entry("os.RemoveAll, a Remove* name", "os", `os.RemoveAll("d")`),
		Entry("anything in os/exec", "os/exec", `exec.Command("x")`),
		Entry("net.Dial, the base of Dial*", "net", `func() error { _, err := net.Dial("tcp", "x"); return err }()`),
		Entry("net.DialTimeout, a Dial* name", "net", `func() error { _, err := net.DialTimeout("tcp", "x", 0); return err }()`),
		Entry("net.Listen, the base of Listen*", "net", `func() error { _, err := net.Listen("tcp", "x"); return err }()`),
		Entry("net.ListenPacket, a Listen* name", "net", `func() error { _, err := net.ListenPacket("udp", "x"); return err }()`),
		Entry("http.Get", "net/http", `func() error { _, err := http.Get("u"); return err }()`),
		Entry("http.Head", "net/http", `func() error { _, err := http.Head("u"); return err }()`),
		Entry("http.Post, the base of Post*", "net/http", `func() error { _, err := http.Post("u", "t", nil); return err }()`),
		Entry("http.PostForm, a Post* name", "net/http", `func() error { _, err := http.PostForm("u", nil); return err }()`),
		Entry("http.Serve, the base of Serve*", "net/http", `http.Serve(nil, nil)`),
		Entry("http.ServeTLS, a Serve* name", "net/http", `http.ServeTLS(nil, nil, "c", "k")`),
		Entry("http.ListenAndServe, the base of ListenAndServe*", "net/http", `http.ListenAndServe("x", nil)`),
		Entry("http.ListenAndServeTLS, a ListenAndServe* name", "net/http", `http.ListenAndServeTLS("x", "c", "k", nil)`),
		Entry("http.DefaultClient.Do", "net/http", `func() error { _, err := http.DefaultClient.Do(nil); return err }()`),
		Entry("Do on a client built in place", "net/http", `func() error { _, err := (&http.Client{}).Do(nil); return err }()`),
		Entry("Do on a client value built in place", "net/http", `func() error { _, err := (http.Client{}).Do(nil); return err }()`),
		Entry("sql.Open", "database/sql", `func() error { _, err := sql.Open("d", "s"); return err }()`),
		Entry("sql.OpenDB", "database/sql", `sql.OpenDB(nil)`),
		Entry("anything in syscall", "syscall", `syscall.Getpid()`),
	)

	DescribeTable("measures a function's nesting",
		func(body string, nesting map[string]any) {
			source := "package p\n\nfunc F(a, b, c bool, v any, ch chan int) {\n" + body + "\n}\n"
			Expect(measure(profileOfSource(source), "shapes", "nesting")).To(Equal(nesting))
		},
		Entry("an if inside an else block, one deeper", "if a {\n} else {\n\tif b {\n\t}\n}", map[string]any{"2": 1.0}),
		Entry("an if inside an else-if, one deeper than the chain", "if a {\n} else if b {\n\tif c {\n\t}\n}", map[string]any{"2": 1.0}),
		Entry("a condition-only for inside a three-clause for", "for i := 0; i < 3; i++ {\n\tfor a {\n\t}\n}", map[string]any{"2": 1.0}),
		Entry("an else-if chain, at its if's depth", "if a {\n} else if b {\n} else if c {\n}", map[string]any{"1": 1.0}),
		Entry("a type switch in a select in a switch", "switch {\ncase a:\n\tselect {\n\tcase <-ch:\n\t\tswitch v.(type) {\n\t\t}\n\t}\n}", map[string]any{"3": 1.0}),
	)

	When("a file holds only plain for loops", func() {
		It("counts each one as a branch point", func() {
			profile := profileOfSource("package p\n\nfunc F(a bool) {\n\tfor i := 0; i < 3; i++ {\n\t}\n\tfor a {\n\t}\n}\n")

			Expect(measure(profile, "shapes", "branch_density")).To(Equal(map[string]any{"2": 1.0}))
		})
	})

	When("a function is declared without a body, as one implemented in assembly is", func() {
		It("is not measured", func() {
			profile := profileOfSource("package p\n\nfunc F() {}\n\nfunc Asm(x int) int\n")

			Expect(measure(profile, "shapes", "nesting")).To(Equal(map[string]any{"0": 1.0}))
			Expect(measure(profile, "dimensions", "entry")).To(Equal(map[string]any{"http_handler": 0.0, "method": 0.0, "function": 1.0}))
		})
	})

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
