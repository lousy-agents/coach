package baseline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectPackArgsEmbedSpanWindowsNotFullFiles() {
	root := multiHiddenMutationFixtureRoot()
	// Pad hot.go so full-file content is much larger than a ±15 window.
	hotPath := filepath.Join(root, "hot.go")
	body, err := os.ReadFile(hotPath)
	Expect(err).NotTo(HaveOccurred())
	var pad strings.Builder
	pad.Write(body)
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&pad, "// pad-line-%d-unique-marker-FULLFILE\n", i)
	}
	Expect(os.WriteFile(hotPath, []byte(pad.String()), 0o644)).To(Succeed())

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "pack-owner",
		SmokeRepoName:    "pack-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
		},
	})

	w := newCaptureWriter()
	_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())

	var checked int
	for _, loop := range observed {
		if loop == nil {
			continue
		}
		checked += expectSpanWindowEvidence(loop)
	}
	Expect(checked).To(BeNumerically(">=", 1), "expected at least one pack item with file content")
}

// expectSpanWindowEvidence requires every non-empty file content in loop's
// hidden_mutation_contextualization args to be a numbered span window rather
// than the padded full file, and returns how many contents it checked.
func expectSpanWindowEvidence(loop *agentloop.Loop) int {
	checked := 0
	for _, c := range loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		for _, content := range hiddenMutationPackContents(c.Args) {
			checked++
			// Span windows are numbered ("  N|..." / "> N|..."); full files are not.
			Expect(content).To(MatchRegexp(`(?m)^[> ]\s*\d+\|`),
				"evidence should be FormatSpanWindow-numbered, not raw full file")
			Expect(content).NotTo(ContainSubstring("pad-line-79-unique-marker-FULLFILE"),
				"default evidence must not embed the entire padded file")
		}
	}
	return checked
}

// hiddenMutationPackContents decodes the non-empty file contents carried by
// one hidden_mutation_contextualization call, whether packed (items[].file)
// or singular (file).
func hiddenMutationPackContents(raw json.RawMessage) []string {
	var args struct {
		Items []struct {
			File struct {
				Content string `json:"content"`
			} `json:"file"`
		} `json:"items"`
		File struct {
			Content string `json:"content"`
		} `json:"file"`
	}
	Expect(json.Unmarshal(raw, &args)).To(Succeed())
	contents := []string{}
	for _, it := range args.Items {
		if it.File.Content != "" {
			contents = append(contents, it.File.Content)
		}
	}
	if args.File.Content != "" {
		contents = append(contents, args.File.Content)
	}
	return contents
}
