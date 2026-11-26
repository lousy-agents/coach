package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var failedPackage = regexp.MustCompile(`(?m)^FAIL[ \t]+(\S+)`)

func failingPackages(output generateOutcome) []string {
	var packages []string
	for _, match := range failedPackage.FindAllStringSubmatch(output.stdout+output.stderr, -1) {
		if strings.Contains(match[1], "/") {
			packages = append(packages, match[1])
		}
	}
	sort.Strings(packages)
	return packages
}

func manifestOf(program generatedProgram) map[string]any {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(program.Path), "manifest.json"))
	Expect(err).NotTo(HaveOccurred())
	manifest := map[string]any{}
	Expect(json.Unmarshal(content, &manifest)).To(Succeed())
	return manifest
}

func followOnField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	followOn, present := template["follow_on"].(map[string]any)
	Expect(present).To(BeTrue(), "the manifest's template names the follow-on")
	return followOn["field"].(string)
}

var _ = Describe("a generated Go program's follow-on slot", func() {
	for _, cell := range programCells {
		It("fails on the first feature's oracle, passes with its own oracle, and is caught when the first field breaks ("+strings.Join(cell.flags(), " ")+") (AC-19)", func() {
			program := generateOne(GinkgoT().TempDir(), append(cell.flags(), "--seed", "9")...)
			work := GinkgoT().TempDir()
			copyTree(readTree(program.Path), work)
			copyTree(readTree(filepath.Join(program.OracleDir, "tests")), work)
			copyTree(readTree(filepath.Join(program.OracleDir, "solution")), work)
			firstDone := goIn(work, "test", "./...")
			Expect(firstDone.exitCode).To(Equal(0), "the first feature is done: "+firstDone.stdout+firstDone.stderr)

			followOnTests := readTree(filepath.Join(program.OracleDir, "followon", "tests"))
			Expect(followOnTests).NotTo(BeEmpty())
			Expect(readTree(program.Path)).NotTo(HaveKey(MatchRegexp(`^tests/`)), "no hidden test is in the tree")
			Expect(os.RemoveAll(filepath.Join(work, "tests", "feature"))).To(Succeed(), "the follow-on changes the entry's contract, so the first feature's hidden tests are replaced by the follow-on's")
			copyTree(followOnTests, work)
			before := goIn(work, "test", "./...")
			Expect(failingPackages(before)).To(HaveLen(1), "only the follow-on package fails: "+before.stdout+before.stderr)
			Expect(failingPackages(before)[0]).To(HaveSuffix("/tests/followon"))

			followOnSolution := readTree(filepath.Join(program.OracleDir, "followon", "solution"))
			Expect(followOnSolution).NotTo(BeEmpty())
			copyTree(followOnSolution, work)
			done := goIn(work, "test", "./...")
			Expect(done.exitCode).To(Equal(0), "the follow-on oracle passes every test: "+done.stdout+done.stderr)
			Expect(goIn(work, "vet", "./...").exitCode).To(Equal(0))

			firstField := regexp.MustCompile(`(\brecord|\bfirst|\bsecond|\bbody|\))\.` + followOnFirstField(manifestOf(program)) + `\b`)
			for path, content := range followOnSolution {
				swapped := firstField.ReplaceAll(content, []byte("${1}."+followOnField(manifestOf(program))))
				Expect(os.WriteFile(filepath.Join(work, filepath.FromSlash(path)), swapped, 0o644)).To(Succeed())
			}
			broken := goIn(work, "test", "./...")
			Expect(failingPackages(broken)).To(HaveLen(1), "reading the first field from the second must fail: "+broken.stdout+broken.stderr)
			Expect(failingPackages(broken)[0]).To(HaveSuffix("/tests/followon"))
			Expect(broken.stdout).NotTo(ContainSubstring("[build failed]"), "it fails by the assertion and not by the compiler")

			secondField := regexp.MustCompile(`(\brecord|\bfirst|\bsecond|\bbody|\))\.` + followOnField(manifestOf(program)) + `\b`)
			for path, content := range followOnSolution {
				swapped := secondField.ReplaceAll(content, []byte("${1}."+followOnFirstField(manifestOf(program))))
				Expect(os.WriteFile(filepath.Join(work, filepath.FromSlash(path)), swapped, 0o644)).To(Succeed())
			}
			brokenSecond := goIn(work, "test", "./...")
			Expect(failingPackages(brokenSecond)).To(HaveLen(1), "reading the second field from the first must fail too: "+brokenSecond.stdout+brokenSecond.stderr)
			Expect(failingPackages(brokenSecond)[0]).To(HaveSuffix("/tests/followon"))
		})
	}

	It("names the follow-on in the manifest and the generator version that emits it (AC-19)", func() {
		program := generateOne(GinkgoT().TempDir(), append(programCells[1].flags(), "--seed", "9")...)
		manifest := manifestOf(program)

		Expect(manifest["program"].(map[string]any)["generator_version"]).To(Equal("go-2"))
		Expect(followOnField(manifest)).NotTo(Equal(followOnFirstField(manifest)))
	})

	It("regenerates a cached program whose follow-on oracle no longer matches its id (AC-19)", func() {
		cacheHome := GinkgoT().TempDir()
		args := append(baseCell.flags(), "--seed", "6")
		first := generateOne(cacheHome, args...)
		solution := readTree(filepath.Join(first.OracleDir, "followon", "solution"))
		var path string
		for candidate := range solution {
			path = candidate
		}
		Expect(path).NotTo(BeEmpty())
		Expect(os.WriteFile(filepath.Join(first.OracleDir, "followon", "solution", path), []byte("package tampered\n"), 0o644)).To(Succeed())

		repaired := generateOne(cacheHome, args...)

		Expect(repaired.Cache).To(Equal("miss"))
		Expect(readTree(filepath.Join(repaired.OracleDir, "followon", "solution"))).To(Equal(solution))
	})
})

func followOnFirstField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	return template["follow_on"].(map[string]any)["first_field"].(string)
}
