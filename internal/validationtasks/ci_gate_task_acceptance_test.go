package validationtasks

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ci-gate is what the PR hook runs. It exists because the hook used to run
// ci-all, and ci-all is the wrong shape for that job once GitHub Actions runs
// the same checks as parallel required jobs.
//
// The measurement that chose the old design no longer holds. Spike K projected
// a warm ci-all at ~41s, which made "the gate re-runs everything itself" both
// the strongest and the cheapest option. Measured on a CCR container it is
// ~910s, against the hook's own 900s timeout -- and GitHub Actions proves a
// strict superset of it in ~426s wall clock, in parallel, on compute that is
// not the session's. A local gate that costs twice the wall clock to prove
// less is not rigour, it is duplicated work charged to the scarcer budget.
//
// What stays local is what only a local check can do: the clean-worktree
// comparison, because GHA validates the pushed commit and cannot see that the
// working tree differs from it. What ci-gate adds is a cheap smoke signal so
// an obvious break costs seconds rather than a PR round trip.

// taskBody spans from a table header to the next one, so it swallows the
// comment block documenting the *following* table. Reading a task's steps out
// of that raw span silently mixes in the neighbour's prose -- which is how the
// exclusion below first failed, matching "go test" inside ci-all's comment.
// Steps are what these assertions are about, so strip comments to get them.
func taskSteps(toml, name string) string {
	var kept []string
	for _, line := range strings.Split(taskBody(toml, name), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// leadingComment returns the contiguous run of comment lines immediately above
// a table header -- where a task's own documentation lives, and where
// taskBody cannot see it.
func leadingComment(toml, name string) string {
	start := strings.Index(toml, "[tasks."+name+"]")
	if start < 0 {
		return ""
	}
	before := strings.Split(toml[:start], "\n")
	var out []string
	for i := len(before) - 2; i >= 0; i-- {
		if !strings.HasPrefix(strings.TrimSpace(before[i]), "#") {
			break
		}
		out = append([]string{before[i]}, out...)
	}
	return strings.Join(out, "\n")
}

var _ = Describe("the pre-PR gate task", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	It("exists", func() {
		Expect(taskBody(toml, "ci-gate")).NotTo(BeEmpty(),
			"the PR hook needs a task scoped to it; pointing the hook at ci-all or ci-fast re-buys the cost this split exists to remove")
	})

	// The whole point is that no test executes here. ci-fast is not a
	// substitute: it runs `test`, which is `go test -race ./...`, and with the
	// sidecar built first that includes pkg/projectmodel's suite -- the single
	// most expensive leg, measured at 451s. Test execution is GHA's job now.
	It("runs no tests", func() {
		body := taskSteps(toml, "ci-gate")
		Expect(body).NotTo(BeEmpty(), "ci-gate must exist for these exclusions to mean anything")
		for _, testTask := range []string{"test", "test-examples", "ci-go", "ci", "ci-fast", "ci-all", "js-ci", "projectmodel-sidecar-acceptance"} {
			Expect(body).NotTo(ContainSubstring(`task = "`+testTask+`"`),
				"%q executes tests; the gate must stay cheap enough that it is never the reason a PR is slow", testTask)
		}
		Expect(body).NotTo(ContainSubstring("go test"),
			"a raw go test line evades the task-name exclusions above")
	})

	// tidy-check is `go mod tidy && git diff --exit-code`, so it rewrites
	// go.mod and go.sum in place. Running it after the hook has verified a
	// clean tree would dirty the very tree the hook just certified, and the PR
	// would publish something the check never saw. GHA runs it inside ci-go,
	// where nothing has certified a tree.
	It("does not run the one check that mutates the tree it is guarding", func() {
		body := taskSteps(toml, "ci-gate")
		// taskSteps returns "" for a missing task, and "" contains nothing --
		// so a bare NotTo(ContainSubstring) here passes most confidently when
		// ci-gate does not exist at all. Same shape as the strings.Index(-1)
		// trap in the step 0 specs.
		Expect(body).NotTo(BeEmpty(), "ci-gate must exist for this exclusion to mean anything")
		Expect(body).NotTo(ContainSubstring(`task = "tidy-check"`),
			"tidy-check mutates go.mod/go.sum, which would invalidate the gate's own clean-worktree finding")
	})

	It("still catches the cheap mechanical breaks", func() {
		body := taskSteps(toml, "ci-gate")
		for _, step := range []string{"gofmt", "go-vet", "acceptance-style-check"} {
			Expect(indexOfStep(body, step)).To(BeNumerically(">", -1),
				"%q costs seconds and is the most common way a PR comes back red; losing it makes the gate pure overhead", step)
		}
	})

	// A narrowed local gate is only safe because the exhaustive suite is a
	// required check on the branch. If that pairing is ever dropped, this file
	// is the note explaining why the gate looks thin.
	It("documents that GitHub Actions is now the exhaustive gate", func() {
		Expect(strings.ToLower(leadingComment(toml, "ci-gate"))).To(SatisfyAny(
			ContainSubstring("gha"), ContainSubstring("github actions"), ContainSubstring("branch protection")),
			"a future reader must not mistake the narrow scope for an oversight and 'restore' ci-all here")
	})
})
