package validationtasks

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ci-gate is the fast local smoke check. It exists because a serial local
// ci-all measured ~910s on a CCR container, while GitHub Actions proves a
// strict superset of it in ~426s wall clock, in parallel, on compute that is
// not the session's. A local run that costs twice the wall clock to prove
// less is not rigour, it is duplicated work charged to the scarcer budget --
// so the exhaustive gate is GHA plus branch protection, and ci-gate is the
// cheap smoke signal in front of it: an obvious break costs seconds rather
// than a PR round trip.

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

var _ = Describe("the ci-gate smoke task", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	It("exists", func() {
		Expect(taskBody(toml, "ci-gate")).NotTo(BeEmpty(),
			"the fast smoke check needs its own task; reaching for ci-all or ci-fast instead re-buys the cost this split exists to remove")
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
	// go.mod and go.sum in place -- a smoke check that dirties the working
	// tree mid-run. GHA runs it inside ci-go, where that does not matter.
	It("does not run the one check that mutates the tree it is guarding", func() {
		body := taskSteps(toml, "ci-gate")
		// taskSteps returns "" for a missing task, and "" contains nothing --
		// so a bare NotTo(ContainSubstring) here passes most confidently when
		// ci-gate does not exist at all.
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
