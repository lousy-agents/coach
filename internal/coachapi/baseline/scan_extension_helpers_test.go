package baseline_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

func expectOnlySemanticsSupportedPathsAnalyzed() {
	root := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "util.ts"), []byte("export const n = 1;\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "Widget.tsx"), []byte("export const W = () => null;\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "notes.md"), []byte("# docs\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "script.py"), []byte("print('no')\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "data.json"), []byte(`{"a":1}`+"\n"), 0o644)).To(Succeed())

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
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
