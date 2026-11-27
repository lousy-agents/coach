package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

// schemaShapes is the closed list of shapes the committed profile schema
// lets a profile name under not_covered.
func schemaShapes() []string {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "internal", "evalpatterns", "profile.schema.json"))
	Expect(err).NotTo(HaveOccurred())
	var schema struct {
		Properties struct {
			NotCovered struct {
				Items struct {
					Properties struct {
						Shape struct {
							Enum []string `json:"enum"`
						} `json:"shape"`
					} `json:"properties"`
				} `json:"items"`
			} `json:"not_covered"`
		} `json:"properties"`
	}
	Expect(json.Unmarshal(raw, &schema)).To(Succeed())
	Expect(schema.Properties.NotCovered.Items.Properties.Shape.Enum).NotTo(BeEmpty())
	return schema.Properties.NotCovered.Items.Properties.Shape.Enum
}

// observing is the committed coach-self profile with every dimension and
// shape emptied, then given only what observe sets.
func observing(observe func(profile map[string]any)) []byte {
	GinkgoHelper()
	return edited(committedProfile(), false, func(profile map[string]any) {
		profile["roles"] = map[string]any{}
		profile["not_covered"] = []any{}
		dimensions := profile["dimensions"].(map[string]any)
		dimensions["layers"], dimensions["depth"] = map[string]any{}, map[string]any{}
		dimensions["entry"] = map[string]any{"function": 0, "method": 0, "http_handler": 0}
		profile["shapes"] = map[string]any{
			"nesting": map[string]any{}, "cognitive": map[string]any{}, "branch_density": map[string]any{},
			"constructor": map[string]any{"assigns_only": 0, "does_work": 0},
		}
		observe(profile)
	})
}

func acceptAnyProfile([]byte) error { return nil }

// weighsBy reports whether the generator's weight tells apart two cells
// that differ only by vary, for a profile that observed one value of it.
func weighsBy(observe func(profile map[string]any), vary func(cell *evalgen.Cell)) {
	GinkgoHelper()
	profile, err := evalgen.ReadProfile(observing(observe), acceptAnyProfile)
	Expect(err).NotTo(HaveOccurred())
	cell, _ := evalgen.CellFromIndex(0)
	other := cell
	vary(&other)
	Expect(profile.Weight(cell)).NotTo(Equal(profile.Weight(other)), "the generator weighs cells by it")
}

// derives reports whether a profile that observed only this shape yields
// the #871 template's fixture.
func derives(observe func(profile map[string]any), template string) {
	GinkgoHelper()
	result, err := antipatternfixtures.Generate(observing(observe), acceptAnyProfile, GinkgoT().TempDir(), "probe",
		antipatternfixtures.Templates(), antipatternfixtures.GoVet)
	Expect(err).NotTo(HaveOccurred())
	var emitted []string
	for _, fixture := range result.Emitted {
		emitted = append(emitted, fixture.ID)
	}
	Expect(emitted).To(ContainElement(template), "a #871 template derives a fixture from it")
}

func shape(profile map[string]any, name string) map[string]any {
	return profile["shapes"].(map[string]any)[name].(map[string]any)
}

func dimension(profile map[string]any, name string) map[string]any {
	return profile["dimensions"].(map[string]any)[name].(map[string]any)
}

// shapeReaders shows, for each shape, the generator operator or the #871
// template that reads it. AC-2.9 lists under not_covered an observed shape
// with neither; every shape in today's closed list has one, so the listing
// has nothing to list, and this row keeps that true as the list grows.
var shapeReaders = map[string]func(){
	"layers": func() {
		weighsBy(func(p map[string]any) { dimension(p, "layers")["4"] = 5 }, func(c *evalgen.Cell) { c.Layers = 4 })
	},
	"packages": func() {
		weighsBy(func(p map[string]any) {
			p["roles"] = map[string]any{"api": map[string]any{"packages": map[string]any{"3": 5}}}
		},
			func(c *evalgen.Cell) { c.Packages = 3 })
	},
	"entry": func() {
		weighsBy(func(p map[string]any) { dimension(p, "entry")["method"] = 5 }, func(c *evalgen.Cell) { c.Entry = "method" })
	},
	"depth": func() {
		weighsBy(func(p map[string]any) { dimension(p, "depth")["3"] = 5 }, func(c *evalgen.Cell) { c.Depth = 3 })
	},
	"nesting":        func() { derives(func(p map[string]any) { shape(p, "nesting")["3"] = 1 }, "arrow-code") },
	"cognitive":      func() { derives(func(p map[string]any) { shape(p, "cognitive")["15"] = 1 }, "spaghetti-conditional") },
	"branch_density": func() { derives(func(p map[string]any) { shape(p, "branch_density")["12"] = 1 }, "god-file") },
	"constructor": func() {
		derives(func(p map[string]any) { shape(p, "constructor")["does_work"] = 1 }, "constructor-does-real-work")
	},
}

var _ = Describe("every shape a profile can name has a generator operator or a #871 template (#872 AC-2.9)", func() {
	It("finds a reader for each shape in the profile schema's closed list", func() {
		for _, name := range schemaShapes() {
			reader, known := shapeReaders[name]
			Expect(known).To(BeTrue(), "the schema names the shape %q, which has neither a generator operator nor a #871 template here, "+
				"so AC-2.9 must list it under not_covered with a count", name)
			By(name)
			reader()
		}
	})
})
