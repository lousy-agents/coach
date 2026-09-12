package projectmodel_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// sidecarOptsWithModeAndCounter extends sidecarOptsWithMode with the fake
// sidecar's --invocation-counter-file flag, letting a spec assert exactly
// how many separate subprocess round trips a call sequence made.
func sidecarOptsWithModeAndCounter(mode, counterFile string) projectmodel.TSSidecarOptions {
	opts := sidecarOptsWithMode(mode)
	opts.Args = append(opts.Args, "--invocation-counter-file="+counterFile)
	return opts
}

// invocationCount reads the fake sidecar's invocation counter file (one line
// appended per subprocess invocation) and reports how many invocations it
// recorded. A missing file (no invocation yet) counts as zero.
func invocationCount(counterFile string) int {
	data, err := os.ReadFile(counterFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		Fail(fmt.Sprintf("reading invocation counter file %s: %v", counterFile, err))
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

var _ = Describe("BuildTypeScriptReachability", func() {
	When("a TS route handler has a resolved call path to a pinned query-shaped sink", func() {
		It("produces a possible_call_reachability ReachabilityFact with structured path, resolved-direct confidence, and the TS algorithm identity", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability"))
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Algorithm).To(Equal("ts-source-sink-registry@1"))
			Expect(result.Algorithm).NotTo(Equal(projectmodel.ReachabilityAlgorithm),
				"expected the TS traversal's own algorithm identity, distinct from Go's go-source-sink-registry@1")

			Expect(result.Facts).To(HaveLen(1), "expected exactly one reachability fact, got %+v", result.Facts)
			fact := result.Facts[0]
			Expect(fact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(fact.Confidence).To(Equal(projectmodel.ReachabilityConfidenceResolvedDirect))
			Expect(fact.Source).To(Equal("file:src/app.ts#getUsers"))
			Expect(fact.Sink).To(Equal("(PrismaClient).findMany"))
			Expect(fact.Path).To(Equal([]projectmodel.ReachabilityStep{
				{NodeID: "file:src/app.ts#getUsers"},
				{NodeID: "(PrismaClient).findMany"},
			}))
			Expect(fact.AlgorithmVersion).To(Equal("ts-source-sink-registry@1"))

			Expect(result.Sources).To(Equal([]string{"file:src/app.ts#getUsers"}),
				"Sources on the TS path is derived from the resolved facts' own Source values, a narrower claim than Go's independently-identified handler set")
			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("the sidecar reports several facts, two sharing one source and a second source out of sorted order", func() {
		It("returns Sources as the deduplicated, sorted set of fact Source values", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability_multi"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(HaveLen(3))
			Expect(result.Sources).To(Equal([]string{
				"file:src/app.ts#createUser",
				"file:src/app.ts#getUsers",
			}), "expected a deduplicated, sorted Sources set, got %v", result.Sources)
		})
	})

	When("the sidecar reports no reachability facts for this snapshot", func() {
		It("returns empty Facts and Sources without error, having achieved complete coverage", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("happy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(BeEmpty())
			Expect(result.Sources).To(BeEmpty())
			Expect(result.Algorithm).To(Equal("ts-source-sink-registry@1"))
			Expect(result.Coverage.Complete).To(BeTrue(),
				"expected complete coverage so this case is distinguishable from the backend-unavailable case below, which also yields zero facts")
		})
	})

	When("the sidecar reports a coverage gap alongside a resolved fact", func() {
		It("still returns the resolved fact but with Coverage.Complete false", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability_gap"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(HaveLen(1))
			Expect(result.Coverage.Complete).To(BeFalse())
		})
	})

	When("the sidecar backend is unavailable", func() {
		It("returns empty Facts and Sources, incomplete coverage, and a project_backend_unavailable diagnostic instead of a Go error", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("crash"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(BeEmpty())
			Expect(result.Sources).To(BeEmpty())
			Expect(result.Coverage.Complete).To(BeFalse())
			_, ok := diagnosticWithCode(result.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", result.Coverage.Diagnostics)
		})
	})
})

var _ = Describe("deriving a ReachabilityResult from an already-built Model (AC-RUN-5: one analyzer invocation per revision)", func() {
	When("a caller calls the public BuildTypeScriptReachability wrapper more than once", func() {
		It("invokes the sidecar once per call, reproducing the independent-round-trip cost this task eliminates", func() {
			counterFile := filepath.Join(GinkgoT().TempDir(), "invocations.log")
			opts := sidecarOptsWithModeAndCounter("reachability", counterFile)

			_, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			_, err = projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(invocationCount(counterFile)).To(Equal(2),
				"each call to the public wrapper legitimately re-builds the Model; this pins the baseline the Model-reuse spec below eliminates and proves the counter instrumentation itself works")
		})
	})

	When("a caller builds the Model once via BuildTypeScriptModelViaSidecar and derives ReachabilityResult from it", func() {
		It("invokes the sidecar exactly once, deriving Facts/Sources/Coverage purely from the already-built Model", func() {
			counterFile := filepath.Join(GinkgoT().TempDir(), "invocations.log")
			opts := sidecarOptsWithModeAndCounter("reachability", counterFile)

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())

			result := projectmodel.BuildTypeScriptReachabilityFromModel(model)
			Expect(result.Algorithm).To(Equal("ts-source-sink-registry@1"))
			Expect(result.Facts).To(HaveLen(1))
			Expect(result.Sources).To(Equal([]string{"file:src/app.ts#getUsers"}))
			Expect(result.Coverage.Complete).To(BeTrue())

			Expect(invocationCount(counterFile)).To(Equal(1),
				"expected exactly one sidecar invocation: BuildTypeScriptReachabilityFromModel must derive purely from the passed-in Model without any sidecar round trip of its own")
		})
	})
})
