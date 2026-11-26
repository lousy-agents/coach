package evalharness_test

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

func report(statuses map[string]int, file string) []byte {
	var mutations []string
	for status, count := range statuses {
		for range count {
			mutations = append(mutations, `{"type":"CONDITIONALS_NEGATION","status":"`+status+`","line":1,"column":1}`)
		}
	}
	return []byte(`{"files":[{"file_name":"` + file + `","mutations":[` + strings.Join(mutations, ",") + `]}]}`)
}

func verdictOf(statuses map[string]int) evalharness.MutationVerdict {
	counts, err := evalharness.ParseMutationReport(report(statuses, "internal/api/entry/entry.go"), []string{"internal/api/entry"})
	Expect(err).NotTo(HaveOccurred())
	return counts.Verdict()
}

func downloaded(cache string) []string {
	var files []string
	Expect(filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !strings.HasSuffix(path, ".lock") {
			files = append(files, path)
		}
		return err
	})).To(Succeed())
	return files
}

var _ = Describe("the mutation gate's verdict (design §6, step 6)", func() {
	It("passes vacuously when no mutant is valid, and says so by recording no score (AC-24)", func() {
		verdict := verdictOf(map[string]int{})

		Expect(verdict.Pass).To(BeTrue())
		Expect(verdict.Valid).To(Equal(0))
		Expect(verdict.Score).To(BeNil())
		Expect(verdict.NotCoveredShare).To(BeNil())
	})

	DescribeTable("scores detected over valid, and rejects below 0.8 or above 10% not covered (AC-24)",
		func(statuses map[string]int, pass bool) {
			verdict := verdictOf(statuses)

			Expect(verdict.Pass).To(Equal(pass), verdict.Reason)
		},
		Entry("a score of exactly 0.8 passes", map[string]int{"KILLED": 4, "LIVED": 1}, true),
		Entry("a score of 0.75 fails", map[string]int{"KILLED": 3, "LIVED": 1}, false),
		Entry("timed out counts as detected", map[string]int{"KILLED": 3, "TIMED OUT": 1, "LIVED": 1}, true),
		Entry("not viable is excluded from valid", map[string]int{"KILLED": 4, "LIVED": 1, "NOT VIABLE": 20}, true),
		Entry("skipped is excluded from valid", map[string]int{"KILLED": 4, "LIVED": 1, "SKIPPED": 20}, true),
		Entry("not covered at exactly 10% of valid passes", map[string]int{"KILLED": 9, "NOT COVERED": 1}, true),
		Entry("not covered above 10% fails although the score is 0.8", map[string]int{"KILLED": 8, "NOT COVERED": 2}, false),
		Entry("not covered counts against the score", map[string]int{"KILLED": 7, "NOT COVERED": 1, "LIVED": 2}, false),
	)

	It("records the score and the share it judged (AC-24)", func() {
		verdict := verdictOf(map[string]int{"KILLED": 9, "NOT COVERED": 1})

		Expect(verdict.Valid).To(Equal(10))
		Expect(*verdict.Score).To(BeNumerically("~", 0.9))
		Expect(*verdict.NotCoveredShare).To(BeNumerically("~", 0.1))
	})

	It("names the failed threshold in the reason (AC-24)", func() {
		Expect(verdictOf(map[string]int{"KILLED": 3, "LIVED": 1}).Reason).To(ContainSubstring("score"))
		Expect(verdictOf(map[string]int{"KILLED": 8, "NOT COVERED": 2}).Reason).To(ContainSubstring("not covered"))
	})

	It("judges only the files of packages reachable from the api surface (AC-24)", func() {
		onRoute := report(map[string]int{"KILLED": 4, "LIVED": 1}, "internal/api/entry/entry.go")
		counts, err := evalharness.ParseMutationReport(onRoute, []string{"internal/api/entry"})
		Expect(err).NotTo(HaveOccurred())
		Expect(counts.Verdict().Valid).To(Equal(5))

		offRoute := report(map[string]int{"LIVED": 9}, "internal/api/other/other.go")
		counts, err = evalharness.ParseMutationReport(offRoute, []string{"internal/api/entry"})
		Expect(err).NotTo(HaveOccurred())
		Expect(counts.Verdict().Valid).To(Equal(0), "a mutant outside the route is not judged")
	})

	It("refuses a report that is not JSON (AC-24)", func() {
		_, err := evalharness.ParseMutationReport([]byte("not json"), nil)

		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("the pinned Gremlins (design §6, step 6; §10)", func() {
	evalModule := filepath.Join("..", "..")

	It("is the tool directive's version, resolved from the module cache and not from PATH (AC-25)", func() {
		gremlins, err := evalharness.ResolveGremlins(evalModule, nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(evalharness.PinnedGremlinsVersion).To(Equal("v0.6.0"))
		Expect(gremlins.Version).To(Equal(evalharness.PinnedGremlinsVersion))
		Expect(gremlins.CheckVersion()).To(Succeed())
		onPath, lookupErr := exec.LookPath("gremlins")
		if lookupErr == nil {
			Expect(gremlins.Binary).NotTo(Equal(onPath), "a gremlins on PATH is not the pinned one")
		}
		output, err := exec.Command("go", "version", "-m", gremlins.Binary).Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(output)).To(ContainSubstring("github.com/go-gremlins/gremlins\tv0.6.0"))
	})

	It("bounds a run by thirty minutes when its caller sets no limit (AC-24)", func() {
		Expect(evalharness.DefaultGremlinsLimit).To(Equal(30 * time.Minute))
	})

	It("refuses a binary built from another Gremlins version (AC-25)", func() {
		gremlins, err := evalharness.ResolveGremlins(evalModule, nil)
		Expect(err).NotTo(HaveOccurred())

		err = evalharness.Gremlins{Binary: gremlins.Binary, Version: "v0.0.1"}.CheckVersion()

		Expect(err).To(MatchError(ContainSubstring("pinned version is v0.0.1")))
	})

	DescribeTable("exits as a missing tool when the module cache lacks it, without fetching it, whatever the caller's environment says (AC-25)",
		func(callerEnv ...string) {
			emptyCache := GinkgoT().TempDir()

			_, err := evalharness.ResolveGremlins(evalModule, append([]string{"GOMODCACHE=" + emptyCache}, callerEnv...))

			Expect(err).To(MatchError(evalharness.ErrGremlinsUnavailable))
			Expect(downloaded(emptyCache)).To(BeEmpty(), "resolving must not download anything")
		},
		Entry("no extra environment"),
		Entry("a named module proxy", "GOPROXY=https://proxy.golang.org,direct"),
		Entry("direct module lookup", "GOPROXY=direct"),
	)
})
