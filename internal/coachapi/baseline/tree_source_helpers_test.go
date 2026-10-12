package baseline_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func expectGitHubTreeSourceBaselineCompletes() {
	const objectSHA = "0123456789abcdef0123456789abcdef01234567"
	mutate := []byte(`package main

type C struct{ N string }

func Mut(c *C, n string) { c.N = n }
`)
	plain := []byte("package main\n\nfunc main() {}\n")
	fx := newGitHubBaselineTreeFixture(objectSHA, map[string][]byte{
		"main.go":   plain,
		"mutate.go": mutate,
		"notes.md":  []byte("# ignored\n"),
	})
	server := fakegithub.NewServer(fx)
	DeferCleanup(server.Close)

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 42,
		PrivateKey:     baselineRSAKey(),
		BaseURL:        server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
		TreeSource: &baseline.GitHubTreeSource{Reader: reader},
		Gateway:    modelgateway.NewStubGateway(),
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
		},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "acme",
		RepoName:  "widgets",
		Ref:       "main",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"GitHubBaselineTreeSource must drive handler end-to-end against fakegithub Contents")
	Expect(completion).NotTo(BeNil())
	Expect(completion.CommitSHA).To(Equal(objectSHA),
		"commit_sha must come from ResolveCommitSHA on the real GitHub adapter")
	Expect(completion.CommitSHA).NotTo(Equal("main"))
	Expect(completion.CommitSHA).NotTo(Equal("local-fixture"))

	Expect(observed).NotTo(BeEmpty())
	var names []string
	for _, loop := range observed {
		names = append(names, handlerSourcedNames(loop.Calls())...)
	}
	Expect(names).To(ContainElement(agentloop.ToolSemanticsAnalyze))
	Expect(names).To(ContainElement(agentloop.ToolCodeSignalReport))

	analyzed := semanticsPathsFromLoops(observed)
	Expect(analyzed).To(HaveKey("main.go"))
	Expect(analyzed).To(HaveKey("mutate.go"))
	Expect(analyzed).NotTo(HaveKey("notes.md"),
		"GitHubBaselineTreeSource must filter to semantics-supported extensions")

	var det int
	for _, f := range w.findings {
		if f.Source == coachapi.FindingSourceDeterministic {
			det++
		}
	}
	Expect(det).To(BeNumerically(">=", 1),
		"Contents-backed tree must produce deterministic findings from mutate.go")
}
