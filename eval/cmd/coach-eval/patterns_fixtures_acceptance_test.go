package main_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// copyStrippingTxt copies the files under source into dest with the .txt
// suffix #871's fixtures carry removed.
func copyStrippingTxt(source, dest string) {
	GinkgoHelper()
	Expect(filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, strings.TrimSuffix(rel, ".txt"))
		Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
		return os.WriteFile(target, raw, 0o644)
	})).To(Succeed())
}

// extractedProfile is the profile of the Go tree at root under label.
func extractedProfile(label, root string) []byte {
	GinkgoHelper()
	reference := evalpatterns.Reference{Label: label, SHA: strings.Repeat("ab", 20), Language: "go", Licence: "MIT"}
	raw, err := evalpatterns.ExtractProfile(reference, root)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// standInProfiles writes, into a profiles directory, the profile of a tree
// holding the bad variant of every #871 fixture.
func standInProfiles() string {
	GinkgoHelper()
	pack := filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns")
	entries, err := os.ReadDir(pack)
	Expect(err).NotTo(HaveOccurred())
	root := filepath.Join(GinkgoT().TempDir(), "stand-in")
	for _, entry := range entries {
		copyStrippingTxt(filepath.Join(pack, entry.Name(), "bad"), filepath.Join(root, entry.Name()))
	}
	profiles := filepath.Join(GinkgoT().TempDir(), "profiles")
	Expect(os.MkdirAll(profiles, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(profiles, "stand-in.profile.json"), extractedProfile("stand-in", root), 0o644)).To(Succeed())
	return profiles
}

func runPatternsFixtures(args ...string) generateOutcome {
	return runGenerateCommand(exec.Command(coachEvalBinary, append([]string{"patterns", "fixtures"}, args...)...))
}

func fixtureNote(dir string) string {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(dir, "expect.json"))
	Expect(err).NotTo(HaveOccurred())
	var expect struct {
		Note string `json:"note"`
	}
	Expect(json.Unmarshal(raw, &expect)).To(Succeed())
	return expect.Note
}

func largestMeasured(profile []byte, shape string) int {
	GinkgoHelper()
	var decoded struct {
		Shapes map[string]map[string]int `json:"shapes"`
	}
	Expect(json.Unmarshal(profile, &decoded)).To(Succeed())
	largest := -1
	for bucket := range decoded.Shapes[shape] {
		value, err := strconv.Atoi(bucket)
		Expect(err).NotTo(HaveOccurred())
		largest = max(largest, value)
	}
	return largest
}

var _ = Describe("coach-eval patterns fixtures (#872 Task 5)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "derived") })

	It("turns a profile of #871's fixtures into fixtures that coach-eval antipatterns runs and reports, beside the hand-authored pack (AC-4.1, AC-4.2)", func() {
		outcome := runPatternsFixtures("--profiles", standInProfiles(), "--out", out)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(ids(filepath.Join(out, "stand-in"))).To(Equal([]string{"arrow-code", "constructor-does-real-work", "god-file", "spaghetti-conditional"}))
		scan := runAntipatterns(filepath.Join(out, "stand-in"), antipatternCoach())
		Expect(scan.exitCode).To(Equal(0), scan.stderr)
		table := decodeTable(scan.stdout)
		Expect(table.Fixtures).To(HaveLen(4))
		Expect(table.Summary.Fail).To(BeZero())
		Expect(coveredFixtureIDs(table)).To(Equal(map[string]bool{"arrow-code": true, "god-file": true, "spaghetti-conditional": true}))
	})

	DescribeTable("writes a bad variant that measures at the value its note records",
		func(id, shape, phrase string) {
			Expect(runPatternsFixtures("--profiles", standInProfiles(), "--out", out).exitCode).To(Equal(0))
			fixture := filepath.Join(out, "stand-in", id)
			bad := filepath.Join(GinkgoT().TempDir(), "bad")
			copyStrippingTxt(filepath.Join(fixture, "bad"), bad)

			value := largestMeasured(extractedProfile("bad", bad), shape)

			Expect(fixtureNote(fixture)).To(ContainSubstring(phrase + " " + strconv.Itoa(value)))
		},
		Entry("arrow code's nesting depth", "arrow-code", "nesting", "nesting depth"),
		Entry("a spaghetti conditional's cognitive complexity", "spaghetti-conditional", "cognitive", "cognitive complexity"),
		Entry("a god file's branch points", "god-file", "branch_density", "branch points"),
	)

	DescribeTable("refuses with exit 2, writing nothing",
		func(arrange func(profiles string), args func(profiles string) []string, message string) {
			profiles := standInProfiles()
			arrange(profiles)

			outcome := runPatternsFixtures(args(profiles)...)

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring(message))
			Expect(out).NotTo(BeADirectory())
		},
		Entry("a profile that fails the committed schema", func(profiles string) {
			path := filepath.Join(profiles, "stand-in.profile.json")
			raw, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(path, []byte(strings.Replace(string(raw), `"label":`, `"path":"pkg/x","label":`, 1)), 0o644)).To(Succeed())
		}, func(profiles string) []string { return []string{"--profiles", profiles, "--out", out} }, "stand-in.profile.json"),
		Entry("no --profiles", func(string) {}, func(string) []string { return []string{"--out", out} }, "--profiles is required"),
		Entry("no --out", func(string) {}, func(profiles string) []string { return []string{"--profiles", profiles} }, "--out is required"),
		Entry("a profiles directory with no profile", func(profiles string) {
			Expect(os.Remove(filepath.Join(profiles, "stand-in.profile.json"))).To(Succeed())
		}, func(profiles string) []string { return []string{"--profiles", profiles, "--out", out} }, "no profile"),
	)
})

func ids(dir string) []string {
	GinkgoHelper()
	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
