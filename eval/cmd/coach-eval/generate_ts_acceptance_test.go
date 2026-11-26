package main_test

import (
	"encoding/json"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type typeScriptPolicy struct {
	Language string `json:"language"`
	Roots    []string
	Layers   []struct {
		Name     string
		Prefixes []string
	}
	ForbiddenImports []struct{ From, To string } `json:"forbidden_imports"`
	RequiredLayer    string                      `json:"required_layer"`
}

var leakageWords = regexp.MustCompile(`(?i)\b(oracle|hidden|FAIL_TO_PASS|trap|seed|solution|operator)\b|tests/feature`)

var _ = Describe("a generated TypeScript program tree", func() {
	cell := programCells[1]
	flags := append(typeScriptCellFlags(cell), "--seed", "4")

	When("generated twice for one cell and seed", func() {
		It("has byte-identical trees, and program_id is the hash of the tree (781 AC-1)", func() {
			first := generateOne(GinkgoT().TempDir(), flags...)
			second := generateOne(GinkgoT().TempDir(), flags...)

			firstTree, secondTree := readTree(first.Path), readTree(second.Path)
			Expect(secondTree).To(Equal(firstTree))
			Expect(first.ProgramID).To(Equal(second.ProgramID))
			Expect(first.ProgramID).To(Equal(treeID(firstTree)))
		})

		It("differs from the Go program of the same cell and seed", func() {
			ts := generateOne(GinkgoT().TempDir(), flags...)
			goProgram := generateOne(GinkgoT().TempDir(), append(cell.flags(), "--seed", "4")...)

			Expect(ts.ProgramID).NotTo(Equal(goProgram.ProgramID))
			Expect(readTree(goProgram.Path)).To(HaveKey("go.mod"))
			Expect(readTree(ts.Path)).NotTo(HaveKey("go.mod"))
		})

		It("is served from the cache the second time", func() {
			cacheHome := GinkgoT().TempDir()
			first := generateOne(cacheHome, flags...)
			second := generateOne(cacheHome, flags...)

			Expect(first.Cache).To(Equal("miss"))
			Expect(second.Cache).To(Equal("hit"))
		})
	})

	When("inspected", func() {
		var tree map[string][]byte

		BeforeEach(func() {
			tree = readTree(generateOne(GinkgoT().TempDir(), flags...).Path)
		})

		It("holds one root tsconfig and a package.json, and no other project file the grader refuses (781 constraint)", func() {
			Expect(filesMatching(tree, `(^|/)tsconfig[^/]*\.json$`)).To(Equal([]string{"tsconfig.json"}))
			Expect(filesMatching(tree, `(^|/)package\.json$`)).To(Equal([]string{"package.json"}))
			config := string(tree["tsconfig.json"])
			for _, refused := range []string{`"references"`, `"paths"`, `"baseUrl"`, `"extends"`} {
				Expect(config).NotTo(ContainSubstring(refused))
			}
			manifest := string(tree["package.json"])
			for _, refused := range []string{`"imports"`, `"exports"`} {
				Expect(manifest).NotTo(ContainSubstring(refused))
			}
		})

		It("imports only by relative, static specifiers that name a file of the tree (781 constraint)", func() {
			for path, source := range tree {
				if !strings.HasSuffix(path, ".ts") {
					continue
				}
				Expect(string(source)).NotTo(MatchRegexp(`\brequire\(|import\(`), path)
				for _, specifier := range specifiersOf(source) {
					if specifier == "node:test" || specifier == "node:assert/strict" {
						continue
					}
					Expect(specifier).To(MatchRegexp(`^\.{1,2}/`), path)
					Expect(strings.Count(specifier, "../")).To(BeNumerically("<=", 1), path+" stays 2 below the deep-import threshold of 3")
					resolved := resolveRelative(path, specifier)
					Expect(tree).To(HaveKey(resolved), path+" imports "+specifier)
				}
			}
		})

		It("declares its language, its layers under src/, one forbidden pair and one required layer in the policy", func() {
			var policy typeScriptPolicy
			Expect(json.Unmarshal(tree[".lousy-agents/coach.json"], &policy)).To(Succeed())

			Expect(policy.Language).To(Equal("typescript"))
			Expect(policy.Roots).To(Equal([]string{"."}))
			Expect(policy.Layers).To(HaveLen(cell.layers))
			for _, layer := range policy.Layers {
				Expect(layer.Prefixes).To(Equal([]string{"src/" + layer.Name}))
				Expect(filesMatching(tree, `^src/`+layer.Name+`/[a-z]+\.ts$`)).To(HaveLen(cell.packages))
			}
			Expect(policy.ForbiddenImports).To(HaveLen(1))
			Expect(policy.RequiredLayer).NotTo(BeEmpty())
		})

		It("carries the self-review hook byte for byte, as the Go program does", func() {
			goTree := readTree(generateOne(GinkgoT().TempDir(), append(cell.flags(), "--seed", "4")...).Path)

			Expect(tree["scripts/coach-self-review.sh"]).To(Equal(goTree["scripts/coach-self-review.sh"]))
		})

		It("names none of the hidden machinery and holds no hidden test", func() {
			for path, source := range tree {
				Expect(leakageWords.FindString(path+"\n"+string(source))).To(BeEmpty(), path)
			}
			Expect(tree).NotTo(HaveKey(MatchRegexp(`^tests/`)))
		})
	})
})

func resolveRelative(from, specifier string) string {
	parts := strings.Split(from, "/")
	parts = parts[:len(parts)-1]
	for _, segment := range strings.Split(specifier, "/") {
		switch segment {
		case ".":
		case "..":
			parts = parts[:len(parts)-1]
		default:
			parts = append(parts, segment)
		}
	}
	return strings.Join(parts, "/")
}
