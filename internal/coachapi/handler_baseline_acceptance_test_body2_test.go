package coachapi_test

import (
	"context"

	"encoding/json"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_handlerBaselineAcceptanceTest_analyzesOnlySemanticsSupportedPathsGoTsTsxAndSki_312() {
	root := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "util.ts"), []byte("export const n = 1;\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "Widget.tsx"), []byte("export const W = () => null;\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "notes.md"), []byte("# docs\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "script.py"), []byte("print('no')\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "data.json"), []byte(`{"a":1}`+"\n"), 0o644)).To(Succeed())

	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "lang-owner",
		SmokeRepoName:    "lang-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop:      func(loop *agentloop.Loop) { observed = append(observed, loop) },
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "lang-owner",
		RepoName:  "lang-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(completion).NotTo(BeNil())
	Expect(observed).NotTo(BeEmpty())

	analyzed := semanticsPathsFromLoops(observed)
	Expect(analyzed).To(HaveKey("main.go"))
	Expect(analyzed).To(HaveKey("util.ts"))
	Expect(analyzed).To(HaveKey("Widget.tsx"))
	Expect(analyzed).NotTo(HaveKey("notes.md"), "markdown is outside the semantics language registry")
	Expect(analyzed).NotTo(HaveKey("script.py"), "python is outside the semantics language registry")
	Expect(analyzed).NotTo(HaveKey("data.json"), "json is outside the semantics language registry")
	Expect(analyzed).To(HaveLen(3), "exactly the three supported-language files must be analyzed")
}

func body_handlerBaselineAcceptanceTest_completesABaselineViaRealListFilesReadFileResolv_508() {
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
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		TreeSource: &coachapi.GitHubBaselineTreeSource{Reader: reader},
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

func semanticsPathsFromLoops(loops []*agentloop.Loop) map[string]struct{} {
	analyzed := map[string]struct{}{}
	for _, loop := range loops {
		for _, path := range semanticsPathsFromLoop(loop) {
			analyzed[path] = struct{}{}
		}
	}
	return analyzed
}

func semanticsPathsFromLoop(loop *agentloop.Loop) []string {
	var paths []string
	for _, c := range loop.Calls() {
		if c.Source != agentloop.CallSourceHandler || c.Name != agentloop.ToolSemanticsAnalyze {
			continue
		}
		var args struct {
			Path string `json:"path"`
		}
		Expect(json.Unmarshal(c.Args, &args)).To(Succeed(), "semantics_analyze args must be JSON with path")
		paths = append(paths, args.Path)
	}
	return paths
}
