package antipatternfixtures_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

// generatorRoles is the generator's role list for each layer count, outermost first.
var generatorRoles = map[int][]string{
	3: {"api", "service", "store"},
	4: {"api", "service", "domain", "store"},
	5: {"api", "service", "domain", "repository", "store"},
}

// layered is a profile that records the given layer-count distribution, the packages each role was seen with, and
// the shapes listed as not covered, and observed no complexity shape.
func layered(layers map[string]int, roles map[string]map[string]int, notCovered ...string) []byte {
	GinkgoHelper()
	var profile map[string]any
	Expect(json.Unmarshal(profileWith(map[string]int{}, map[string]int{}, map[string]int{}, 0), &profile)).To(Succeed())
	profile["dimensions"].(map[string]any)["layers"] = layers
	observed := map[string]any{}
	for role, packages := range roles {
		observed[role] = map[string]any{"packages": packages}
	}
	profile["roles"] = observed
	items := []any{}
	for _, shape := range notCovered {
		items = append(items, map[string]any{"shape": shape, "reason": "out_of_range", "count": 1})
	}
	profile["not_covered"] = items
	raw, err := json.Marshal(profile)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// evenly gives every role of a layer count the same package distribution.
func evenly(layers int, packages map[string]int) map[string]map[string]int {
	roles := map[string]map[string]int{}
	for _, role := range generatorRoles[layers] {
		roles[role] = packages
	}
	return roles
}

type policyFile struct {
	Layers []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"forbidden_imports"`
	RequiredLayer string `json:"required_layer"`
}

func policyOf(out, id, variant string) policyFile {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(out, id, variant, ".lousy-agents", "coach.json.txt"))
	Expect(err).NotTo(HaveOccurred(), "the policy is stored as coach.json.txt, as #871's layer fixtures store it")
	var policy policyFile
	Expect(json.Unmarshal(raw, &policy)).To(Succeed())
	return policy
}

// packageDirs are the directories under internal/<role>/ that hold a Go file, by role.
func packageDirs(out, id, variant string) map[string][]string {
	GinkgoHelper()
	dirs := map[string][]string{}
	for name := range tree(filepath.Join(out, id, variant)) {
		parts := strings.Split(name, "/")
		if len(parts) == 4 && parts[0] == "internal" && strings.HasSuffix(parts[3], ".go.txt") {
			dirs[parts[1]] = append(dirs[parts[1]], parts[2])
		}
	}
	for role := range dirs {
		sort.Strings(dirs[role])
		dirs[role] = unique(dirs[role])
	}
	return dirs
}

func unique(sorted []string) []string {
	var out []string
	for _, name := range sorted {
		if len(out) == 0 || out[len(out)-1] != name {
			out = append(out, name)
		}
	}
	return out
}

var _ = Describe("deriving the layer fixtures from a profile's layers (#900, #872 AC-4.1)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "secret-label") })

	layerIDs := []string{"dependency-rule-breach", "skipped-layer"}

	When("the profile records an ordered layer count of 3 to 5 with the packages of its roles", func() {
		It("emits both layer fixtures, in #871's layout, with the policy stored as coach.json.txt (AC-4.2)", func() {
			result, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1})), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(ids(out)).To(Equal(layerIDs))
			Expect(result.Emitted).To(ConsistOf(
				antipatternfixtures.Emitted{ID: "dependency-rule-breach", Value: 4},
				antipatternfixtures.Emitted{ID: "skipped-layer", Value: 4}))
			for _, id := range layerIDs {
				Expect(tree(filepath.Join(out, id))).To(SatisfyAll(
					HaveKey("expect.json"), HaveKey("bad/go.mod.txt"), HaveKey("good/go.mod.txt"),
					HaveKey("bad/.lousy-agents/coach.json.txt"), HaveKey("good/.lousy-agents/coach.json.txt")))
			}
		})

		DescribeTable("scales to the most frequent layer count from 3 to 5, ties to the larger, and ignores a count outside the range",
			func(layers map[string]int, want int) {
				_, err := generate(layered(layers, evenly(want, map[string]int{"1": 1})), out)

				Expect(err).NotTo(HaveOccurred())
				for _, id := range layerIDs {
					Expect(policyOf(out, id, "bad").Layers).To(HaveLen(want))
					Expect(readExpect(out, id).Note).To(ContainSubstring("layer count " + itoa(want)))
				}
			},
			Entry("the most frequent", map[string]int{"3": 1, "4": 5, "5": 2}, 4),
			Entry("a tie goes to the larger", map[string]int{"3": 2, "5": 2}, 5),
			Entry("a count of 3, the lowest", map[string]int{"3": 1}, 3),
			Entry("a count over 5 is ignored, though it is the most frequent", map[string]int{"4": 1, "9": 7}, 4),
			Entry("a count under 3 is ignored, though it is the most frequent", map[string]int{"3": 1, "2": 7}, 3),
		)

		DescribeTable("holds each layer to the most frequent package count the roles observed, ties to the larger, capped at 3",
			func(packages map[string]int, want, from int) {
				_, err := generate(layered(map[string]int{"4": 1}, evenly(4, packages)), out)

				Expect(err).NotTo(HaveOccurred())
				for _, id := range layerIDs {
					for _, variant := range []string{"bad", "good"} {
						dirs := packageDirs(out, id, variant)
						Expect(dirs).To(HaveLen(4))
						for _, role := range generatorRoles[4] {
							Expect(dirs[role]).To(HaveLen(want), "%s/%s/%s", id, variant, role)
						}
					}
					note := readExpect(out, id).Note
					Expect(note).To(ContainSubstring(itoa(want) + " packages per layer"))
					if from != want {
						Expect(note).To(ContainSubstring("capped at " + itoa(want) + " from " + itoa(from)))
					} else {
						Expect(note).NotTo(ContainSubstring("capped"))
					}
				}
			},
			Entry("the most frequent", map[string]int{"1": 1, "2": 4, "3": 2}, 2, 2),
			Entry("a tie goes to the larger", map[string]int{"1": 3, "2": 3}, 2, 2),
			Entry("a count over the cap is capped, and the note says from what", map[string]int{"9": 2, "1": 1}, 3, 9),
		)

		It("counts the packages of every role of the layer count together, and ignores a role the layer count does not use", func() {
			roles := map[string]map[string]int{"api": {"2": 1}, "service": {"2": 1}, "store": {"3": 1}, "repository": {"1": 9}}

			_, err := generate(layered(map[string]int{"3": 1}, roles), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(packageDirs(out, "skipped-layer", "bad")["api"]).To(HaveLen(2), "2 is the most frequent of the three roles of 3 layers; repository is not one of them")
		})

		It("holds one package a layer, and says so, when the profile records no packages for the roles", func() {
			_, err := generate(layered(map[string]int{"3": 1}, map[string]map[string]int{}), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(packageDirs(out, "dependency-rule-breach", "bad")["api"]).To(HaveLen(1))
			Expect(readExpect(out, "dependency-rule-breach").Note).To(ContainSubstring("1 packages per layer"))
			Expect(readExpect(out, "dependency-rule-breach").Note).To(ContainSubstring("no packages were observed"))
		})

		DescribeTable("orders the layers by forbidden pairs: every inner layer is forbidden to import every outer one, once",
			func(layers int) {
				_, err := generate(layered(map[string]int{itoa(layers): 1}, evenly(layers, map[string]int{"1": 1})), out)

				Expect(err).NotTo(HaveOccurred())
				roles := generatorRoles[layers]
				var want []string
				for outer := range roles {
					for inner := outer + 1; inner < len(roles); inner++ {
						want = append(want, roles[inner]+">"+roles[outer])
					}
				}
				for _, id := range layerIDs {
					for _, variant := range []string{"bad", "good"} {
						policy := policyOf(out, id, variant)
						var names, got []string
						for _, layer := range policy.Layers {
							names = append(names, layer.Name)
							Expect(layer.Prefixes).To(Equal([]string{"internal/" + layer.Name}))
						}
						for _, pair := range policy.ForbiddenImports {
							got = append(got, pair.From+">"+pair.To)
						}
						Expect(names).To(Equal(roles))
						Expect(got).To(ConsistOf(want))
					}
				}
			},
			Entry("3 layers", 3), Entry("4 layers", 4), Entry("5 layers", 5),
		)

		It("requires the second layer in the skipped-layer policy only, the same in both variants", func() {
			_, err := generate(layered(map[string]int{"5": 1}, evenly(5, map[string]int{"1": 1})), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(policyOf(out, "skipped-layer", "bad").RequiredLayer).To(Equal("service"))
			Expect(policyOf(out, "skipped-layer", "good").RequiredLayer).To(Equal("service"))
			Expect(policyOf(out, "dependency-rule-breach", "bad").RequiredLayer).To(BeEmpty())
			Expect(policyOf(out, "dependency-rule-breach", "good").RequiredLayer).To(BeEmpty())
		})

		DescribeTable("declares each expected finding from the template, whatever the layer count and package count",
			func(layers, packages int) {
				_, err := generate(layered(map[string]int{itoa(layers): 1}, evenly(layers, map[string]int{itoa(packages): 1})), out)
				Expect(err).NotTo(HaveOccurred())

				Expect(badFindings(readExpect(out, "dependency-rule-breach"))).To(Equal([]finding{{
					RuleID: "architecture.layer_violation", Path: "internal/store/orders/reach.go",
					Subject: "architecture.layer_violation:internal/store/orders->internal/api/orders"}}))
				Expect(badFindings(readExpect(out, "skipped-layer"))).To(Equal([]finding{{
					RuleID: "architecture.layer_bypass", Path: "internal/api/orders/handle.go",
					Subject: "architecture.layer_bypass:service:example.com/layers/internal/api/orders.Handle->(*database/sql.DB).Query"}}))
				for _, id := range layerIDs {
					expect := readExpect(out, id)
					Expect(expect.Status).To(Equal("covered"))
					Expect(string(expect.Expect.Good)).To(Equal(`"none"`))
					Expect(expect).To(SatisfyAll(
						HaveField("ID", id), HaveField("Seed", "profile:"+profileIdentity),
						HaveField("Source", SatisfyAll(ContainSubstring("#871"), ContainSubstring(id)))))
				}
			},
			Entry("3 layers, 1 package", 3, 1), Entry("4 layers, 2 packages", 4, 2), Entry("5 layers, 3 packages", 5, 3),
		)

		It("builds a bad variant that differs from its good variant in the breach or the bypass alone", func() {
			_, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1})), out)
			Expect(err).NotTo(HaveOccurred())

			bad, good := tree(filepath.Join(out, "dependency-rule-breach", "bad")), tree(filepath.Join(out, "dependency-rule-breach", "good"))
			Expect(bad).To(HaveKey("internal/store/orders/reach.go.txt"))
			Expect(good).NotTo(HaveKey("internal/store/orders/reach.go.txt"))
			delete(bad, "internal/store/orders/reach.go.txt")
			Expect(bad).To(Equal(good))
			Expect(tree(filepath.Join(out, "skipped-layer", "bad"))["internal/api/orders/handle.go.txt"]).
				NotTo(Equal(tree(filepath.Join(out, "skipped-layer", "good"))["internal/api/orders/handle.go.txt"]))
		})

		It("writes nothing of the reference, and the same bytes from the same profile twice (AC-4.3a, AC-4.6)", func() {
			again := filepath.Join(GinkgoT().TempDir(), "again")
			raw := layered(map[string]int{"5": 1}, evenly(5, map[string]int{"3": 1}))
			_, err := generate(raw, out)
			Expect(err).NotTo(HaveOccurred())
			_, err = generate(raw, again)
			Expect(err).NotTo(HaveOccurred())

			Expect(tree(again)).To(Equal(tree(out)))
			Expect(allFiles(out)).To(HaveEach(Not(ContainSubstring("secret-label"))))
			Expect(allContents(out)).To(HaveEach(Not(ContainSubstring("secret-label"))))
		})
	})

	When("the profile records no layer the generator can scale", func() {
		DescribeTable("emits neither layer fixture",
			func(raw []byte) {
				result, err := generate(raw, out)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Emitted).To(BeEmpty())
				Expect(out).NotTo(BeADirectory())
			},
			Entry("no layer count at all", layered(map[string]int{}, evenly(4, map[string]int{"2": 1}))),
			Entry("only a layer count over 5", layered(map[string]int{"9": 1}, evenly(4, map[string]int{"2": 1}))),
			Entry("only a layer count under 3", layered(map[string]int{"2": 3}, evenly(3, map[string]int{"2": 1}))),
			Entry("a layer count whose bucket counts nothing", layered(map[string]int{"4": 0}, evenly(4, map[string]int{"2": 1}))),
			Entry("the shape coach-self records: 9 layers, no roles, layers and packages listed as not covered",
				layered(map[string]int{"9": 1}, map[string]map[string]int{}, "layers", "packages")),
		)

		It("emits neither when not_covered lists layers, though a layer count in range was also recorded", func() {
			result, err := generate(layered(map[string]int{"4": 3, "9": 1}, evenly(4, map[string]int{"2": 1}), "layers"), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(BeEmpty())
		})

		It("still emits both when not_covered lists only other shapes", func() {
			result, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1}), "packages", "depth", "nesting"), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(HaveLen(2))
		})
	})
})

var _ = Describe("the deletion-safety guards on a layer fixture, whose policy sits in a nested dot directory (#900)", func() {
	var root, sibling, kept string
	BeforeEach(func() {
		parent := GinkgoT().TempDir()
		root = filepath.Join(parent, "derived")
		sibling = filepath.Join(parent, "sibling.txt")
		kept = filepath.Join(root, "other-label", "fixture", "bad", ".lousy-agents", "coach.json.txt")
		Expect(os.MkdirAll(filepath.Dir(kept), 0o755)).To(Succeed())
		Expect(os.WriteFile(kept, []byte("kept\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(sibling, []byte("kept\n"), 0o644)).To(Succeed())
	})

	expectRootAndSiblingKept := func() {
		GinkgoHelper()
		Expect(sibling).To(BeAnExistingFile())
		Expect(kept).To(BeAnExistingFile(), "another label's fixtures survive, nested policy included")
	}

	It("clears only its own label on a rerun, nested policy files included, and keeps another label's", func() {
		raw := layered(map[string]int{"4": 1}, evenly(4, map[string]int{"1": 1}))
		_, err := antipatternfixtures.Generate(raw, acceptAll, root, "secret-label", antipatternfixtures.Templates(), antipatternfixtures.GoVet)
		Expect(err).NotTo(HaveOccurred())
		first := filepath.Join(root, "secret-label", "skipped-layer", "bad", ".lousy-agents", "coach.json.txt")
		Expect(first).To(BeAnExistingFile())

		_, err = antipatternfixtures.Generate(profileWith(map[string]int{"4": 1}, map[string]int{}, map[string]int{}, 0), acceptAll, root, "secret-label", antipatternfixtures.Templates(), antipatternfixtures.GoVet)

		Expect(err).NotTo(HaveOccurred())
		Expect(first).NotTo(BeAnExistingFile(), "a layer fixture no longer observed leaves nothing behind, nested policy included")
		Expect(filepath.Join(root, "secret-label", "skipped-layer")).NotTo(BeADirectory())
		expectRootAndSiblingKept()
	})

	DescribeTable("refuses a layered profile with exit 2 before removing anything",
		func(label string) {
			raw := layered(map[string]int{"4": 1}, evenly(4, map[string]int{"1": 1}))

			_, err := antipatternfixtures.Generate(raw, acceptAll, root, label, antipatternfixtures.Templates(), antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(2))
			expectRootAndSiblingKept()
		},
		Entry("the label ..", ".."), Entry("the label .", "."), Entry("an empty label", ""), Entry("a label with a slash", "other-label/fixture"),
	)

	It("leaves a layer fixture whose variant fails go vet out, writes the others, and exits 3 naming it", func() {
		templates := antipatternfixtures.Templates()
		var broken antipatternfixtures.Template
		for _, template := range templates {
			if template.ID == "skipped-layer" {
				broken = template
			}
		}
		Expect(broken.ID).To(Equal("skipped-layer"), "the layer templates are among Templates()")
		good := broken.Good
		broken.Good = func(value int) map[string]string { return withUnusedVariable(good(value)) }
		templates = append(templates[:0:0], broken)
		for _, template := range antipatternfixtures.Templates() {
			if template.ID != "skipped-layer" {
				templates = append(templates, template)
			}
		}
		raw := layered(map[string]int{"3": 1}, evenly(3, map[string]int{"1": 1}))

		result, err := antipatternfixtures.Generate(raw, acceptAll, root, "secret-label", templates, antipatternfixtures.GoVet)

		Expect(exitOf(err)).To(Equal(3))
		Expect(err.Error()).To(ContainSubstring("skipped-layer"))
		Expect(filepath.Join(root, "secret-label", "skipped-layer")).NotTo(BeADirectory())
		Expect(ids(filepath.Join(root, "secret-label"))).To(Equal([]string{"dependency-rule-breach"}))
		Expect(result.Emitted).To(HaveLen(1))
		expectRootAndSiblingKept()
	})
})
