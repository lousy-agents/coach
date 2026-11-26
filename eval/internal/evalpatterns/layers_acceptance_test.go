package evalpatterns_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// layeredPolicy names three layers out of order and under names of its
// own; only the forbidden pairs say that handlers sit above logic, which
// sits above db.
const layeredPolicy = `{"schema_version":"1","roots":["."],"layers":[` +
	`{"name":"db","prefixes":["pkg/db"]},` +
	`{"name":"handlers","prefixes":["pkg/handlers"]},` +
	`{"name":"logic","prefixes":["pkg/logic"]},` +
	`{"name":"empty","prefixes":["pkg/empty"]}],` +
	`"forbidden_imports":[{"from":"db","to":"logic"},{"from":"db","to":"handlers"},{"from":"logic","to":"handlers"},` +
	`{"from":"empty","to":"handlers"}]}`

func pkgFile(name string) string { return "package " + name + "\n\nfunc helper() {}\n" }

var _ = Describe("the layers and roles of a profile (AC-2.1, AC-2.2, AC-2.8)", func() {
	When("the checkout's policy orders three layers by its forbidden pairs", func() {
		It("records three layers and each layer's packages under the generator's role, never the layer's name", func() {
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": layeredPolicy,
				"pkg/handlers/a/a.go":      pkgFile("a"),
				"pkg/handlers/b/b.go":      pkgFile("b"),
				"pkg/logic/logic.go":       pkgFile("logic"),
				"pkg/db/x/x.go":            pkgFile("x"),
				"pkg/db/y/y.go":            pkgFile("y"),
				"pkg/db/z/z.go":            pkgFile("z"),
				"tools/outside/outside.go": pkgFile("outside"),
			})

			Expect(measure(profile, "dimensions", "layers")).To(Equal(map[string]any{"3": 1.0}))
			Expect(profile["roles"]).To(Equal(map[string]any{
				"api":     map[string]any{"packages": map[string]any{"2": 1.0}},
				"service": map[string]any{"packages": map[string]any{"1": 1.0}},
				"store":   map[string]any{"packages": map[string]any{"3": 1.0}},
			}))
			Expect(profile["not_covered"]).To(ContainElement(map[string]any{"shape": "packages", "count": 1.0}))
			Expect(profile["not_covered"]).NotTo(ContainElement(HaveKeyWithValue("shape", "layers")))
			raw := profileJSON(profile)
			for _, name := range []string{"handlers", "logic", "db", "empty", "outside"} {
				Expect(string(raw)).NotTo(ContainSubstring(`"` + name + `"`))
			}
		})
	})

	DescribeTable("orders the layers by the pairs between layers that hold a package, each pair counted once",
		func(extraPairs string) {
			policy := strings.Replace(layeredPolicy, `"forbidden_imports":[`, `"forbidden_imports":[`+extraPairs, 1)
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": policy,
				"pkg/handlers/a/a.go":      pkgFile("a"), "pkg/handlers/b/b.go": pkgFile("b"),
				"pkg/logic/l1/l.go": pkgFile("l1"), "pkg/logic/l2/l.go": pkgFile("l2"),
				"pkg/db/x/x.go": pkgFile("x"), "pkg/db/y/y.go": pkgFile("y"),
			})

			Expect(profile["roles"]).To(HaveKey("store"))
			Expect(profile["not_covered"]).To(Equal([]any{}))
		},
		Entry("a pair listed twice", `{"from":"db","to":"logic"},`),
		Entry("a pair into a layer that holds no package", `{"from":"db","to":"empty"},`),
		Entry("a pair from a layer to itself", `{"from":"db","to":"db"},`),
	)

	When("a package directory shares a layer prefix's leading characters but not its path", func() {
		It("is not in that layer", func() {
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": layeredPolicy,
				"pkg/handlers/a/a.go":      pkgFile("a"), "pkg/handlers/b/b.go": pkgFile("b"),
				"pkg/logic/l1/l.go": pkgFile("l1"), "pkg/logic/l2/l.go": pkgFile("l2"),
				"pkg/db/x/x.go": pkgFile("x"), "pkg/db/y/y.go": pkgFile("y"),
				"pkg/dbtools/t/t.go": pkgFile("t"), "pkg/dbtools/u/u.go": pkgFile("u"), "pkg/dbtools/v/v.go": pkgFile("v"),
			})

			Expect(profile["roles"].(map[string]any)["store"]).To(Equal(map[string]any{"packages": map[string]any{"2": 1.0}}))
		})
	})

	When("one layer's prefix sits inside another's", func() {
		It("puts a package in the layer with the longest matching prefix", func() {
			nested := `{"schema_version":"1","roots":["."],"layers":[` +
				`{"name":"outer","prefixes":["pkg"]},{"name":"mid","prefixes":["pkg/mid"]},{"name":"inner","prefixes":["pkg/mid/inner"]}],` +
				`"forbidden_imports":[{"from":"inner","to":"mid"},{"from":"inner","to":"outer"},{"from":"mid","to":"outer"}]}`
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": nested,
				"pkg/o1/o.go":              pkgFile("o1"), "pkg/o2/o.go": pkgFile("o2"),
				"pkg/mid/m1/m.go": pkgFile("m1"), "pkg/mid/m2/m.go": pkgFile("m2"), "pkg/mid/m3/m.go": pkgFile("m3"),
				"pkg/mid/inner/i1/i.go": pkgFile("i1"), "pkg/mid/inner/i2/i.go": pkgFile("i2"),
				"pkg/mid/inner/i3/i.go": pkgFile("i3"), "pkg/mid/inner/i4/i.go": pkgFile("i4"),
			})

			Expect(profile["roles"]).To(Equal(map[string]any{
				"api":     map[string]any{"packages": map[string]any{"2": 1.0}},
				"service": map[string]any{"packages": map[string]any{"3": 1.0}},
				"store":   map[string]any{"packages": map[string]any{"4": 1.0}},
			}))
		})
	})

	When("the policy's forbidden pairs leave two layers unordered", func() {
		It("records the layer count, no roles, and the layers as not covered", func() {
			unordered := `{"schema_version":"1","roots":["."],"layers":[` +
				`{"name":"a","prefixes":["pkg/a"]},{"name":"b","prefixes":["pkg/b"]},{"name":"c","prefixes":["pkg/c"]}],` +
				`"forbidden_imports":[{"from":"c","to":"a"}]}`
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": unordered,
				"pkg/a/a1/a.go":            pkgFile("a1"), "pkg/a/a2/a.go": pkgFile("a2"),
				"pkg/b/b1/b.go": pkgFile("b1"), "pkg/b/b2/b.go": pkgFile("b2"),
				"pkg/c/c1/c.go": pkgFile("c1"), "pkg/c/c2/c.go": pkgFile("c2"),
			})

			Expect(measure(profile, "dimensions", "layers")).To(Equal(map[string]any{"3": 1.0}))
			Expect(profile["roles"]).To(Equal(map[string]any{}))
			Expect(profile["not_covered"]).To(ContainElement(map[string]any{"shape": "layers", "count": 1.0}))
		})
	})

	When("the policy orders six layers, more than the generator builds", func() {
		It("records six layers, no roles, and the layers as not covered rather than clamped", func() {
			files := chainedLayers("l1", "l2", "l3", "l4", "l5", "l6")
			profile := profileOfFiles(files)

			Expect(measure(profile, "dimensions", "layers")).To(Equal(map[string]any{"6": 1.0}))
			Expect(profile["roles"]).To(Equal(map[string]any{}))
			Expect(profile["not_covered"]).To(Equal([]any{map[string]any{"shape": "layers", "count": 1.0}}))
		})
	})

	When("the checkout's policy is not JSON", func() {
		It("refuses the entry with exit 2, naming the label and the policy", func() {
			root := standInTree()
			writeFile(filepath.Join(root, ".lousy-agents", "coach.json"), "{")

			_, err := evalpatterns.ExtractProfile(standInReference(), root)

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "ExtractProfile returned %v", err)
			Expect(refusal.Exit).To(Equal(2))
			Expect(err.Error()).To(HavePrefix(`reference "stand-in": `))
			Expect(err.Error()).To(ContainSubstring(".lousy-agents/coach.json"))
		})
	})

	DescribeTable("maps four or five ordered layers to the generator's roles, outermost first",
		func(names []string, counts []int, roles []string) {
			files := chainedLayers(names...)
			for i, name := range names {
				for extra := 3; extra <= counts[i]; extra++ {
					files["pkg/"+name+"/p"+strconv.Itoa(extra)+"/p.go"] = pkgFile("p" + strconv.Itoa(extra))
				}
				if counts[i] == 1 {
					delete(files, "pkg/"+name+"/p2/p.go")
				}
			}
			want := map[string]any{}
			for i, role := range roles {
				want[role] = map[string]any{"packages": map[string]any{strconv.Itoa(counts[i]): 1.0}}
			}

			Expect(profileOfFiles(files)["roles"]).To(Equal(want))
		},
		Entry("four layers", []string{"l1", "l2", "l3", "l4"}, []int{2, 3, 4, 5},
			[]string{"api", "service", "domain", "store"}),
		Entry("five layers", []string{"l1", "l2", "l3", "l4", "l5"}, []int{1, 2, 3, 4, 5},
			[]string{"api", "service", "domain", "repository", "store"}),
	)

	When("a layer lists a longer prefix before a shorter one, and another layer's prefix falls between them", func() {
		It("puts a package under both of the first layer's prefixes in that layer, by its longest prefix", func() {
			policy := `{"schema_version":"1","roots":["."],"layers":[` +
				`{"name":"x","prefixes":["pkg/aa/bb","pkg"]},{"name":"z","prefixes":["pkg/aa"]},{"name":"w","prefixes":["lib/w"]}],` +
				`"forbidden_imports":[{"from":"z","to":"x"},{"from":"w","to":"x"},{"from":"w","to":"z"}]}`
			profile := profileOfFiles(map[string]string{
				".lousy-agents/coach.json": policy,
				"pkg/aa/bb/x1/x.go":        pkgFile("x1"), "pkg/aa/bb/x2/x.go": pkgFile("x2"),
				"pkg/aa/z1/z.go": pkgFile("z1"), "pkg/aa/z2/z.go": pkgFile("z2"), "pkg/aa/z3/z.go": pkgFile("z3"),
				"lib/w/w1/w.go": pkgFile("w1"), "lib/w/w2/w.go": pkgFile("w2"),
				"lib/w/w3/w.go": pkgFile("w3"), "lib/w/w4/w.go": pkgFile("w4"),
			})

			Expect(profile["roles"]).To(Equal(map[string]any{
				"api":     map[string]any{"packages": map[string]any{"2": 1.0}},
				"service": map[string]any{"packages": map[string]any{"3": 1.0}},
				"store":   map[string]any{"packages": map[string]any{"4": 1.0}},
			}))
		})
	})

	When("the checkout holds no production Go file", func() {
		It("records no layer, and lists neither layers nor packages as not covered", func() {
			profile := profileOfFiles(map[string]string{"p/p_test.go": "package p\n"})

			Expect(measure(profile, "dimensions", "layers")).To(Equal(map[string]any{}))
			Expect(profile["not_covered"]).To(Equal([]any{}))
		})
	})

	DescribeTable("refuses a checkout whose policy or go.mod is not a regular file, with exit 2, naming it, without reading through it",
		func(name string) {
			root := standInTree()
			secret := filepath.Join(GinkgoT().TempDir(), "secret.txt")
			writeFile(secret, "hunter2\n")
			Expect(os.MkdirAll(filepath.Join(root, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(os.Remove(filepath.Join(root, filepath.FromSlash(name)))).To(Or(Succeed(), MatchError(os.ErrNotExist)))
			Expect(os.Symlink(secret, filepath.Join(root, filepath.FromSlash(name)))).To(Succeed())

			_, err := evalpatterns.ExtractProfile(standInReference(), root)

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "ExtractProfile returned %v", err)
			Expect(refusal.Exit).To(Equal(2))
			Expect(err.Error()).To(ContainSubstring(name))
			Expect(err.Error()).To(ContainSubstring("not a regular file"))
			Expect(err.Error()).NotTo(ContainSubstring("hunter2"))
		},
		Entry("a symlinked policy", ".lousy-agents/coach.json"),
		Entry("a symlinked go.mod", "go.mod"),
	)

	When("the checkout has no policy", func() {
		It("takes one layer per top-level package directory, with no roles", func() {
			profile := profileOfFiles(map[string]string{
				"a/x/x.go": pkgFile("x"), "a/y/y.go": pkgFile("y"),
				"b/b.go":   pkgFile("b"),
				"c/d/d.go": pkgFile("d"), "c/e/e.go": pkgFile("e"), "c/f/f.go": pkgFile("f"),
			})

			Expect(measure(profile, "dimensions", "layers")).To(Equal(map[string]any{"3": 1.0}))
			Expect(profile["roles"]).To(Equal(map[string]any{}))
			Expect(profile["not_covered"]).To(Equal([]any{map[string]any{"shape": "packages", "count": 1.0}}))
		})
	})
})

var _ = Describe("the static call depth of a profile's entries (AC-2.4, AC-2.8)", func() {
	It("follows same-package calls and calls into local packages, from each entry, by the longest shortest path", func() {
		profile := profileOfFiles(map[string]string{
			"a/a.go": "package a\n\nimport \"example.invalid/one/b\"\n\nfunc Entry() { helper() }\n\nfunc helper() { b.Call(); helper() }\n\n" +
				"type T struct{ x interface{ N() } }\n\nfunc (t T) M() { t.x.N() }\n",
			"b/b.go": "package b\n\nimport \"strings\"\n\nfunc Call() { deeper(); _ = strings.ToUpper(\"x\") }\n\nfunc deeper() {}\n",
		})

		Expect(measure(profile, "dimensions", "depth")).To(Equal(map[string]any{"3": 1.0, "1": 1.0, "0": 1.0}))
		Expect(profile["not_covered"]).To(ContainElement(map[string]any{"shape": "depth", "count": 2.0}))
	})

	It("follows a call into the module's root package", func() {
		profile := profileOfFiles(map[string]string{
			"root.go": "package one\n\nfunc Base() { leaf() }\n\nfunc leaf() {}\n",
			"a/a.go":  "package a\n\nimport \"example.invalid/one\"\n\nfunc Entry() { one.Base() }\n",
		})

		Expect(measure(profile, "dimensions", "depth")).To(Equal(map[string]any{"2": 1.0, "1": 1.0}))
	})

	DescribeTable("reads the module path however go.mod spells its module line",
		func(goMod string) {
			profile := profileOfFiles(map[string]string{
				"go.mod": goMod,
				"a/a.go": "package a\n\nimport \"example.invalid/one/b\"\n\nfunc Entry() { b.Call() }\n",
				"b/b.go": "package b\n\nfunc Call() {}\n",
			})

			Expect(measure(profile, "dimensions", "depth")).To(Equal(map[string]any{"1": 1.0, "0": 1.0}))
		},
		Entry("a quoted module path", "module \"example.invalid/one\"\n\ngo 1.26.0\n"),
		Entry("an indented module line", "  module example.invalid/one\n\ngo 1.26.0\n"),
	)

	It("extracts a checkout with no go.mod, following only same-package calls", func() {
		root := filepath.Join(GinkgoT().TempDir(), "nomod")
		writeFile(filepath.Join(root, "a", "a.go"), "package a\n\nfunc Entry() { helper() }\n\nfunc helper() {}\n")

		profile := decodeProfile(extract(root))

		Expect(measure(profile, "dimensions", "depth")).To(Equal(map[string]any{"1": 1.0}))
	})

	It("lists the out-of-range shapes in a fixed order: depth, layers, packages", func() {
		profile := profileOfFiles(map[string]string{"a/a.go": "package a\n\nfunc Entry() {}\n"})

		Expect(profile["not_covered"]).To(Equal([]any{
			map[string]any{"shape": "depth", "count": 1.0},
			map[string]any{"shape": "layers", "count": 1.0},
			map[string]any{"shape": "packages", "count": 1.0},
		}))
	})
})

// chainedLayers is a module whose policy orders the named layers, outermost
// first, by forbidding each inner layer to import every outer one. Each
// layer holds two packages.
func chainedLayers(names ...string) map[string]string {
	files := map[string]string{}
	var layers, pairs []string
	for i, name := range names {
		layers = append(layers, `{"name":"`+name+`","prefixes":["pkg/`+name+`"]}`)
		files["pkg/"+name+"/p1/p.go"] = pkgFile("p1")
		files["pkg/"+name+"/p2/p.go"] = pkgFile("p2")
		for _, outer := range names[:i] {
			pairs = append(pairs, `{"from":"`+name+`","to":"`+outer+`"}`)
		}
	}
	files[".lousy-agents/coach.json"] = `{"schema_version":"1","roots":["."],"layers":[` + strings.Join(layers, ",") +
		`],"forbidden_imports":[` + strings.Join(pairs, ",") + `]}`
	return files
}
