package coachapi

import (
	"context"
	"encoding/json"

	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func codesignalReportViaLoop(ctx context.Context, loop *agentloop.Loop, fileChanges []codesignal.FileChange, repo, revision string) (*codesignal.Report, error) {
	csArgs, err := json.Marshal(struct {
		Files      []codesignal.FileChange `json:"files"`
		Baseline   bool                    `json:"baseline"`
		Repository string                  `json:"repository"`
		Revision   string                  `json:"revision"`
	}{
		Files:      fileChanges,
		Baseline:   true,
		Repository: repo,
		Revision:   revision,
	})
	if err != nil {
		return nil, err
	}
	rawReport, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport, csArgs)
	if err != nil {
		return nil, fmt.Errorf("coachapi: codesignal_report: %w", err)
	}
	var report codesignal.Report
	if err := json.Unmarshal(rawReport, &report); err != nil {
		return nil, fmt.Errorf("coachapi: decoding codesignal_report: %w", err)
	}
	return &report, nil
}
