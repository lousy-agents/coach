package coachapi_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
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
		(&sigbodyhandlerBaselineJudgmentPackAcceptanceTestembedsSpanWi{checked: &checked, loop: loop}).call()

	}
	Expect(checked).To(BeNumerically(">=", 1), "expected at least one pack item with file content")
}
