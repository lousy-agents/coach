package coachapi_test

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
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineJudgmentPackAcceptanceTest_embedsSpanWindowEvidenceInPackArgsRatherThanFull_145() {
	root := multiHiddenMutationFixtureRoot()

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
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
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
		for _, c := range loop.Calls() {
			if c.Name != rubrics.IDHiddenMutationContextualization {
				continue
			}
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
			Expect(json.Unmarshal(c.Args, &args)).To(Succeed())
			contents := []string{}
			for _, it := range args.Items {
				contents = append(contents, it.File.Content)
			}
			if args.File.Content != "" {
				contents = append(contents, args.File.Content)
			}
			for _, content := range contents {
				if content == "" {
					continue
				}
				checked++

				Expect(content).To(MatchRegexp(`(?m)^[> ]\s*\d+\|`),
					"evidence should be FormatSpanWindow-numbered, not raw full file")
				Expect(content).NotTo(ContainSubstring("pad-line-79-unique-marker-FULLFILE"),
					"default evidence must not embed the entire padded file")
			}
		}
	}
	Expect(checked).To(BeNumerically(">=", 1), "expected at least one pack item with file content")
}
