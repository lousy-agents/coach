package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// committedProfile is the coach-self profile on main.
func committedProfile() []byte {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "patterns", "profiles", "coach-self.profile.json"))
	Expect(err).NotTo(HaveOccurred())
	return raw
}

func profileIdentity(raw []byte) string {
	GinkgoHelper()
	var profile struct {
		Identity string `json:"identity"`
	}
	Expect(json.Unmarshal(raw, &profile)).To(Succeed())
	return profile.Identity
}

func writeProfile(raw []byte) string {
	GinkgoHelper()
	path := filepath.Join(GinkgoT().TempDir(), "coach-self.profile.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

// manifestProgram is the program object of a generated program's manifest.
func manifestProgram(program generatedProgram) map[string]any {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(program.Path), "manifest.json"))
	Expect(err).NotTo(HaveOccurred())
	var manifest map[string]any
	Expect(json.Unmarshal(raw, &manifest)).To(Succeed())
	return manifest["program"].(map[string]any)
}

func cellsOf(programs []generatedProgram) []generatedCell {
	cells := make([]generatedCell, len(programs))
	for i, program := range programs {
		cells[i] = program.Cell
	}
	return cells
}

var _ = Describe("coach-eval generate --profile (#872 Task 4)", func() {
	It("generates --count distinct cells drawn by the profile, reproducibly, each manifest naming the profile (AC-3.1, AC-3.4)", func() {
		raw := committedProfile()
		profile := writeProfile(raw)

		first := runGenerate(GinkgoT().TempDir(), nil, "--profile", profile, "--count", "3", "--seed", "7", "--no-mutation")
		again := runGenerate(GinkgoT().TempDir(), nil, "--profile", profile, "--count", "3", "--seed", "7", "--no-mutation")

		Expect(first.exitCode).To(Equal(0), first.stderr)
		programs := decodeAllGenerated(first.stdout)
		Expect(programs).To(HaveLen(3))
		Expect(cellsOf(decodeAllGenerated(again.stdout))).To(Equal(cellsOf(programs)))
		seen := map[generatedCell]bool{}
		for _, program := range programs {
			Expect(seen).NotTo(HaveKey(program.Cell), "a cell is drawn once")
			seen[program.Cell] = true
			Expect(manifestProgram(program)).To(HaveKeyWithValue("profile", profileIdentity(raw)))
		}
	})

	It("writes no profile into a manifest generated without one (AC-3.5)", func() {
		program := generateOne(GinkgoT().TempDir(), "--cell", "1000", "--seed", "7", "--no-mutation")

		Expect(manifestProgram(program)).NotTo(HaveKey("profile"))
	})

	It("keeps a profile run and a run without one apart in the cache", func() {
		cache := GinkgoT().TempDir()
		profile := writeProfile(committedProfile())
		drawn := decodeAllGenerated(runGenerate(cache, nil, "--profile", profile, "--count", "1", "--seed", "7", "--no-mutation").stdout)
		Expect(drawn).To(HaveLen(1))
		cell := drawn[0].Cell
		flags := programCell{layers: cell.Layers, packages: cell.Packages, entry: cell.Entry, depth: cell.Depth,
			field: cell.Field, direction: cell.Direction, trap: cell.Trap}.flags()

		plain := generateOne(cache, append(flags, "--seed", "7", "--no-mutation")...)
		again := decodeAllGenerated(runGenerate(cache, nil, "--profile", profile, "--count", "1", "--seed", "7", "--no-mutation").stdout)

		Expect(plain.Cache).To(Equal("miss"), "a run without a profile does not reuse the profile run's entry")
		Expect(manifestProgram(plain)).NotTo(HaveKey("profile"))
		Expect(again[0].Cache).To(Equal("hit"))
		Expect(manifestProgram(again[0])).To(HaveKey("profile"))
	})

	DescribeTable("refuses with exit 2",
		func(args func(profile string) []string, message string) {
			profile := writeProfile(committedProfile())

			outcome := runGenerate(GinkgoT().TempDir(), nil, args(profile)...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stdout)
			Expect(outcome.stderr).To(ContainSubstring(message))
			Expect(outcome.stdout).To(BeEmpty())
		},
		Entry("a profile for the held-out panel (AC-3.3)", func(p string) []string {
			return []string{"--profile", p, "--count", "1", "--panel", "held_out"}
		}, "--profile is refused for the held_out panel"),
		Entry("a profile with --cell", func(p string) []string { return []string{"--profile", p, "--count", "1", "--cell", "3"} }, "--cell"),
		Entry("a profile with --tuning", func(p string) []string { return []string{"--profile", p, "--count", "1", "--tuning"} }, "--tuning"),
		Entry("a profile with a dimension", func(p string) []string { return []string{"--profile", p, "--count", "1", "--layers", "3"} }, "--layers"),
		Entry("a profile without --count", func(p string) []string { return []string{"--profile", p} }, "--count"),
		Entry("--count without a profile", func(string) []string { return []string{"--count", "2"} }, "--profile"),
		Entry("a count of 0", func(p string) []string { return []string{"--profile", p, "--count", "0"} }, "--count"),
		Entry("a count over the cell space", func(p string) []string { return []string{"--profile", p, "--count", "1729"} }, "--count"),
		Entry("a profile file that does not exist", func(string) []string {
			return []string{"--profile", filepath.Join(GinkgoT().TempDir(), "missing.profile.json"), "--count", "1"}
		}, "missing.profile.json"),
		Entry("a profile that fails the committed schema (AC-3.6)", func(string) []string {
			raw := strings.Replace(string(committedProfile()), `"label":`, `"path":"pkg/x","label":`, 1)
			return []string{"--profile", writeProfile([]byte(raw)), "--count", "1"}
		}, "does not validate"),
		Entry("a profile from another extractor version", func(string) []string {
			raw := strings.Replace(string(committedProfile()), `"extractor_version":"0.2.0"`, `"extractor_version":"0.1.0"`, 1)
			return []string{"--profile", writeProfile([]byte(raw)), "--count", "1"}
		}, `extractor_version "0.1.0"`),
	)
})
