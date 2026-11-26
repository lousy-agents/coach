package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// layeredStandIn writes a Go tree of four ordered layers of two packages each, with a policy that forbids every inner
// layer to import every outer one, and returns the directory of its profile.
func layeredStandIn() string {
	GinkgoHelper()
	roles := []string{"api", "service", "domain", "store"}
	root := filepath.Join(GinkgoT().TempDir(), "layered")
	write := func(name, content string) {
		GinkgoHelper()
		Expect(os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)).To(Succeed())
	}
	write("go.mod", "module example.com/stand\n\ngo 1.26.0\n")
	var layers, pairs []string
	for i, role := range roles {
		layers = append(layers, fmt.Sprintf(`{"name":%q,"prefixes":["internal/%s"]}`, role, role))
		for _, outer := range roles[:i] {
			pairs = append(pairs, fmt.Sprintf(`{"from":%q,"to":%q}`, role, outer))
		}
		for _, name := range []string{"orders", "billing"} {
			write("internal/"+role+"/"+name+"/"+name+".go", "package "+name+"\n\nfunc Describe() string { return \""+role+"\" }\n")
		}
	}
	write(".lousy-agents/coach.json", `{"schema_version":"1","roots":["."],"layers":[`+strings.Join(layers, ",")+`],"forbidden_imports":[`+strings.Join(pairs, ",")+`]}`)
	profiles := filepath.Join(GinkgoT().TempDir(), "profiles")
	Expect(os.MkdirAll(profiles, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(profiles, "layered-stand-in.profile.json"), extractedProfile("layered-stand-in", root), 0o644)).To(Succeed())
	return profiles
}

var _ = Describe("the layer fixtures a profile derives, run against the real coach (#900, #872 AC-4.1)", func() {
	It("emits dependency-rule-breach and skipped-layer for a profile of four layers of two packages, and coach reports each declared finding on the bad variant and nothing on the good one", func() {
		profiles := layeredStandIn()
		out := filepath.Join(GinkgoT().TempDir(), "derived")

		generated := runPatternsFixtures("--profiles", profiles, "--out", out)
		Expect(generated.exitCode).To(Equal(0), generated.stderr)
		Expect(ids(filepath.Join(out, "layered-stand-in"))).To(ContainElements("dependency-rule-breach", "skipped-layer"))

		scan := runAntipatterns(filepath.Join(out, "layered-stand-in"), antipatternCoach())

		Expect(scan.exitCode).To(Equal(0), scan.stderr)
		Expect(coveredFixtureIDs(decodeTable(scan.stdout))).To(SatisfyAll(
			HaveKeyWithValue("dependency-rule-breach", true), HaveKeyWithValue("skipped-layer", true)))
	})

	It("emits no layer fixture for the committed coach-self profile, which records 9 layers", func() {
		profiles := filepath.Join(repositoryRoot(), "eval", "patterns", "profiles")
		out := filepath.Join(GinkgoT().TempDir(), "derived")

		generated := runPatternsFixtures("--profiles", profiles, "--out", out)

		Expect(generated.exitCode).To(Equal(0), generated.stderr)
		Expect(ids(filepath.Join(out, "coach-self"))).NotTo(ContainElements("dependency-rule-breach"))
		Expect(ids(filepath.Join(out, "coach-self"))).NotTo(ContainElements("skipped-layer"))
	})
})
