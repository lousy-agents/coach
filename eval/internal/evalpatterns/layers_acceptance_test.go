package evalpatterns_test

import (
	"errors"
	"path/filepath"

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
			names := []string{"l1", "l2", "l3", "l4", "l5", "l6"}
			policy := `{"schema_version":"1","roots":["."],"layers":[`
			files := map[string]string{}
			var pairs []string
			for i, name := range names {
				if i > 0 {
					policy += ","
				}
				policy += `{"name":"` + name + `","prefixes":["pkg/` + name + `"]}`
				files["pkg/"+name+"/p1/p.go"] = pkgFile("p1")
				files["pkg/"+name+"/p2/p.go"] = pkgFile("p2")
				for _, outer := range names[:i] {
					pairs = append(pairs, `{"from":"`+name+`","to":"`+outer+`"}`)
				}
			}
			policy += `],"forbidden_imports":[`
			for i, pair := range pairs {
				if i > 0 {
					policy += ","
				}
				policy += pair
			}
			files[".lousy-agents/coach.json"] = policy + `]}`

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

	It("lists the out-of-range shapes in a fixed order: depth, layers, packages", func() {
		profile := profileOfFiles(map[string]string{"a/a.go": "package a\n\nfunc Entry() {}\n"})

		Expect(profile["not_covered"]).To(Equal([]any{
			map[string]any{"shape": "depth", "count": 1.0},
			map[string]any{"shape": "layers", "count": 1.0},
			map[string]any{"shape": "packages", "count": 1.0},
		}))
	})
})
