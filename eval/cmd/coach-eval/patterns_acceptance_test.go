package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type patternsReference struct {
	Label    string `json:"label"`
	SHA      string `json:"sha"`
	Language string `json:"language"`
	Licence  string `json:"licence"`
}

func writePatternsManifest(references ...patternsReference) string {
	GinkgoHelper()
	raw, err := json.Marshal(map[string]any{"schema_version": "1", "references": references})
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(GinkgoT().TempDir(), "references.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

func runPatterns(args ...string) generateOutcome {
	return runGenerateCommand(exec.Command(coachEvalBinary, append([]string{"patterns"}, args...)...))
}

// patternsLines decodes the one JSON line per entry the command prints.
func patternsLines(stdout string) []map[string]any {
	GinkgoHelper()
	var lines []map[string]any
	for _, text := range strings.Split(strings.TrimSpace(stdout), "\n") {
		var line map[string]any
		Expect(json.Unmarshal([]byte(text), &line)).To(Succeed(), text)
		lines = append(lines, line)
	}
	return lines
}

func profileAt(out, label string) map[string]any {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(out, label+".profile.json"))
	Expect(err).NotTo(HaveOccurred())
	var profile map[string]any
	Expect(json.Unmarshal(raw, &profile)).To(Succeed())
	return profile
}

var _ = Describe("coach-eval patterns (#872 Task 3)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "profiles") })

	When("every entry of the manifest has a clone", func() {
		It("writes one profile per entry under its label, prints one line per entry, and exits 0 (AC-1.1)", func() {
			onePath, oneSHA := censusRepo(goRepoFiles(complexFunctions(1)))
			twoPath, twoSHA := censusRepo(goRepoFiles(complexFunctions(2)))
			manifest := writePatternsManifest(
				patternsReference{Label: "one", SHA: oneSHA, Language: "go", Licence: "MIT"},
				patternsReference{Label: "two", SHA: twoSHA, Language: "go", Licence: "Apache-2.0"},
			)

			outcome := runPatterns("--manifest", manifest, "--clone", "one="+onePath, "--clone", "two="+twoPath, "--out", out)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(profileAt(out, "one")).To(HaveKeyWithValue("licence", "MIT"))
			Expect(profileAt(out, "two")["shapes"].(map[string]any)["cognitive"]).To(Equal(map[string]any{"16": 2.0}))
			lines := patternsLines(outcome.stdout)
			Expect(lines).To(HaveLen(2))
			Expect(lines[0]).To(HaveKeyWithValue("label", "one"))
			Expect(lines[0]).To(HaveKeyWithValue("exit", 0.0))
			Expect(lines[0]).To(HaveKeyWithValue("identity", profileAt(out, "one")["identity"]))
		})

		It("writes byte-identical profiles on two runs, and leaves the clone as it was (AC-1.10)", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			manifest := writePatternsManifest(patternsReference{Label: "one", SHA: sha, Language: "go", Licence: "MIT"})
			second := filepath.Join(GinkgoT().TempDir(), "again")

			Expect(runPatterns("--manifest", manifest, "--clone", "one="+path, "--out", out).exitCode).To(Equal(0))
			Expect(runPatterns("--manifest", manifest, "--clone", "one="+path, "--out", second).exitCode).To(Equal(0))

			first, err := os.ReadFile(filepath.Join(out, "one.profile.json"))
			Expect(err).NotTo(HaveOccurred())
			again, err := os.ReadFile(filepath.Join(second, "one.profile.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(again).To(Equal(first))
			Expect(isolatedGit(path, "status", "--porcelain", "--ignored")).To(BeEmpty())
		})
	})

	When("an entry is refused or does not parse", func() {
		It("still writes every other entry's profile, and exits with the worst code: 3 over 2", func() {
			goodPath, goodSHA := censusRepo(goRepoFiles(complexFunctions(1)))
			brokenPath, brokenSHA := censusRepo(map[string]string{"go.mod": "module example.com/b\n\ngo 1.26.0\n", "p/p.go": "package p\n\nfunc broken( {\n"})
			manifest := writePatternsManifest(
				patternsReference{Label: "good", SHA: goodSHA, Language: "go", Licence: "MIT"},
				patternsReference{Label: "broken", SHA: brokenSHA, Language: "go", Licence: "MIT"},
				patternsReference{Label: "unlicensed", SHA: goodSHA, Language: "go", Licence: " "},
			)

			outcome := runPatterns("--manifest", manifest, "--clone", "good="+goodPath, "--clone", "broken="+brokenPath,
				"--clone", "unlicensed="+goodPath, "--out", out)

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			Expect(profileAt(out, "good")).To(HaveKeyWithValue("label", "good"))
			Expect(filepath.Join(out, "broken.profile.json")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(out, "unlicensed.profile.json")).NotTo(BeAnExistingFile())
			Expect(outcome.stderr).To(ContainSubstring(`reference "broken": file p/p.go does not parse`))
			Expect(outcome.stderr).To(ContainSubstring(`reference "unlicensed": licence is required`))
			Expect(patternsLines(outcome.stdout)).To(ConsistOf(
				And(HaveKeyWithValue("label", "good"), HaveKeyWithValue("exit", 0.0)),
				And(HaveKeyWithValue("label", "broken"), HaveKeyWithValue("exit", 3.0)),
				And(HaveKeyWithValue("label", "unlicensed"), HaveKeyWithValue("exit", 2.0)),
			))
		})

		It("refuses an entry given no clone with exit 2, naming its label, and still writes the others", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			manifest := writePatternsManifest(
				patternsReference{Label: "one", SHA: sha, Language: "go", Licence: "MIT"},
				patternsReference{Label: "unclonedentry", SHA: sha, Language: "go", Licence: "MIT"},
			)

			outcome := runPatterns("--manifest", manifest, "--clone", "one="+path, "--out", out)

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring(`reference "unclonedentry": no --clone was given for it`))
			Expect(profileAt(out, "one")).To(HaveKeyWithValue("label", "one"))
		})
	})

	DescribeTable("refuses a command line it cannot use with exit 2, before reading any clone",
		func(args func(manifest, clone string) []string, message string) {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			manifest := writePatternsManifest(patternsReference{Label: "one", SHA: sha, Language: "go", Licence: "MIT"})

			outcome := runPatterns(args(manifest, path)...)

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring(message))
			Expect(filepath.Join(out, "one.profile.json")).NotTo(BeAnExistingFile())
		},
		Entry("no --manifest", func(_, clone string) []string { return []string{"--clone", "one=" + clone, "--out", out} }, "--manifest is required"),
		Entry("no --out", func(manifest, clone string) []string {
			return []string{"--manifest", manifest, "--clone", "one=" + clone}
		}, "--out is required"),
		Entry("a --clone without a label", func(manifest, clone string) []string {
			return []string{"--manifest", manifest, "--clone", clone, "--out", out}
		}, "must be LABEL=PATH"),
		Entry("a --clone for a label the manifest does not hold", func(manifest, clone string) []string {
			return []string{"--manifest", manifest, "--clone", "one=" + clone, "--clone", "other=" + clone, "--out", out}
		}, `--clone names "other", which the manifest does not hold`),
		Entry("a label given two clones", func(manifest, clone string) []string {
			return []string{"--manifest", manifest, "--clone", "one=" + clone, "--clone", "one=" + clone, "--out", out}
		}, `--clone names "one" twice`),
		Entry("an unknown flag", func(manifest, clone string) []string {
			return []string{"--manifest", manifest, "--clone", "one=" + clone, "--out", out, "--network"}
		}, "network"),
		Entry("a manifest that does not load", func(_, clone string) []string {
			return []string{"--manifest", filepath.Join(GinkgoT().TempDir(), "missing.json"), "--clone", "one=" + clone, "--out", out}
		}, "reference manifest"),
	)
})
