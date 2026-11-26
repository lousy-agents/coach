package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var baseCell = programCell{layers: 4, packages: 3, entry: "function", depth: 3, field: "scalar", direction: "read", trap: "layering"}

type policyFile struct {
	SchemaVersion string   `json:"schema_version"`
	Roots         []string `json:"roots"`
	Layers        []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"forbidden_imports"`
	RequiredLayer string `json:"required_layer"`
}

var _ = Describe("coach-eval generate for a Go program", func() {
	var cacheHome string

	BeforeEach(func() {
		cacheHome = GinkgoT().TempDir()
	})

	When("a cell and a seed are given", func() {
		It("emits a program whose id is the hash of its tree (AC-1)", func() {
			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "7")...)

			tree := readTree(program.Path)
			Expect(program.ProgramID).To(Equal(treeID(tree)))
			Expect(tree).To(HaveKey("go.mod"))
			Expect(tree).To(HaveKey("scripts/coach-self-review.sh"))
		})

		It("emits a policy with the cell's layers, one forbidden edge and one required layer (AC-2)", func() {
			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "7")...)

			var policy policyFile
			Expect(json.Unmarshal(readTree(program.Path)[".lousy-agents/coach.json"], &policy)).To(Succeed())
			Expect(policy.SchemaVersion).To(Equal("1"))
			Expect(policy.Roots).To(Equal([]string{"."}))
			Expect(policy.Layers).To(HaveLen(baseCell.layers))
			Expect(policy.ForbiddenImports).To(HaveLen(1))
			Expect(policy.ForbiddenImports[0].From).To(Equal(policy.Layers[0].Name))
			Expect(policy.ForbiddenImports[0].To).To(Equal(policy.Layers[len(policy.Layers)-1].Name))
			Expect(policy.RequiredLayer).To(Equal(policy.Layers[1].Name))
		})

		It("emits layers times packages directories of source (AC-2)", func() {
			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "7")...)

			var policy policyFile
			tree := readTree(program.Path)
			Expect(json.Unmarshal(tree[".lousy-agents/coach.json"], &policy)).To(Succeed())
			directories := map[string]bool{}
			for path := range tree {
				for _, layer := range policy.Layers {
					prefix := layer.Prefixes[0] + "/"
					if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
						directories[filepath.ToSlash(filepath.Dir(path))] = true
					}
				}
			}
			Expect(directories).To(HaveLen(baseCell.layers*baseCell.packages), "one source directory per package")
		})

		It("keeps every production function branch-free and free of pointer returns, so every threshold margin holds by construction (AC-3)", func() {
			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "7")...)

			for path, content := range readTree(program.Path) {
				if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					Expect(productionShapeViolations(path, content)).To(BeEmpty(), path)
				}
			}
		})

		It("copies the self-review script byte for byte from docs/cli-codesignal.md (AC-4)", func() {
			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "7")...)

			docs, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "cli-codesignal.md"))
			Expect(err).NotTo(HaveOccurred())
			script := string(readTree(program.Path)["scripts/coach-self-review.sh"])
			Expect(string(docs)).To(ContainSubstring("```sh\n" + script + "```"))
		})
	})

	When("the same cell and seed are generated twice by separate processes", func() {
		It("emits byte-identical trees and the same program id (AC-5)", func() {
			first := generateOne(cacheHome, append(baseCell.flags(), "--seed", "11")...)
			second := generateOne(GinkgoT().TempDir(), append(baseCell.flags(), "--seed", "11")...)

			Expect(second.ProgramID).To(Equal(first.ProgramID))
			Expect(readTree(second.Path)).To(Equal(readTree(first.Path)))
		})

		It("emits a different program for another seed (AC-5)", func() {
			first := generateOne(cacheHome, append(baseCell.flags(), "--seed", "11")...)
			second := generateOne(cacheHome, append(baseCell.flags(), "--seed", "12")...)

			Expect(second.ProgramID).NotTo(Equal(first.ProgramID))
		})
	})

	When("a cell is named by its index", func() {
		It("emits the program the same dimensions name (AC-6)", func() {
			byIndex := generateOne(cacheHome, "--cell", "0", "--seed", "3")
			byFlags := generateOne(cacheHome, "--layers", "3", "--packages", "2", "--entry", "function",
				"--depth", "2", "--field", "scalar", "--direction", "read", "--trap", "layering", "--seed", "3")

			Expect(byIndex.ProgramID).To(Equal(byFlags.ProgramID))
		})
	})

	When("the program is already in the cache", func() {
		It("reports a hit, and repairs a cached tree that no longer matches its id (AC-7)", func() {
			args := append(baseCell.flags(), "--seed", "5")
			first := generateOne(cacheHome, args...)
			Expect(first.Cache).To(Equal("miss"))

			Expect(generateOne(cacheHome, args...).Cache).To(Equal("hit"))

			Expect(os.WriteFile(filepath.Join(first.Path, "go.mod"), []byte("corrupted\n"), 0o644)).To(Succeed())
			repaired := generateOne(cacheHome, args...)
			Expect(repaired.ProgramID).To(Equal(first.ProgramID))
			Expect(repaired.Cache).To(Equal("miss"))
			Expect(treeID(readTree(repaired.Path))).To(Equal(first.ProgramID))
		})

		It("regenerates when --no-cache is given (AC-7)", func() {
			args := append(baseCell.flags(), "--seed", "5")
			generateOne(cacheHome, args...)

			Expect(generateOne(cacheHome, append(args, "--no-cache")...).Cache).To(Equal("miss"))
		})
	})

	When("--out names a directory", func() {
		It("also writes a copy of the tree there, and never writes into the repository (AC-8)", func() {
			out := GinkgoT().TempDir()

			program := generateOne(cacheHome, append(baseCell.flags(), "--seed", "5", "--out", out)...)

			Expect(readTree(filepath.Join(out, program.ProgramID))).To(Equal(readTree(program.Path)))
			Expect(program.Path).To(HavePrefix(cacheHome))
		})
	})
})
