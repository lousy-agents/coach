package coachapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func judgeBaselineViaLoop(ctx context.Context, loop *agentloop.Loop, files []loadedBaselineFile, detFindings []JobFinding) ([]JobFinding, []JobDiagnostic, error) {
	byPath := make(map[string]loadedBaselineFile, len(files))
	fileMetas := make([]rubrics.FileMeta, 0, len(files))
	for _, f := range files {
		byPath[f.Path] = f
		fileMetas = append(fileMetas, rubrics.FileMeta{
			Path:     f.Path,
			Language: string(f.Language),
		})
	}

	agentFindings, diagnostics, err := judgeHiddenMutationFindings(ctx, loop, byPath, detFindings)
	if err != nil {
		return nil, nil, err
	}

	cohesionFindings, cohesionDiags, err := judgeChangeCohesion(ctx, loop, fileMetas, detFindings)
	if err != nil {
		return nil, nil, err
	}
	agentFindings = append(agentFindings, cohesionFindings...)
	diagnostics = append(diagnostics, cohesionDiags...)
	return agentFindings, diagnostics, nil
}

func judgeHiddenMutationFindings(ctx context.Context, loop *agentloop.Loop, byPath map[string]loadedBaselineFile, detFindings []JobFinding) ([]JobFinding, []JobDiagnostic, error) {
	var (
		agentFindings []JobFinding
		diagnostics   []JobDiagnostic
	)
	for _, f := range detFindings {
		if f.Source != FindingSourceDeterministic {
			continue
		}
		sig, ok := hiddenMutationSignal(f.Payload)
		if !ok {
			continue
		}
		lf, found := byPath[sig.Path]
		if !found {
			lf = loadedBaselineFile{Path: sig.Path}
		}
		args, err := json.Marshal(map[string]any{
			"finding": json.RawMessage(f.Payload),
			"file": rubrics.FileContext{
				Path:     lf.Path,
				Language: string(lf.Language),
				Content:  lf.Content,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDHiddenMutationContextualization, args)
		if err != nil {
			return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDHiddenMutationContextualization, err)
		}
		// Discriminate agent payload_hash by the deterministic signal's hash so
		// identical stub/live judgments across N hidden-mutation signals do not
		// collide on UNIQUE (job_id, attempt, source, rubric_id, payload_hash).
		af, d, err := jobOutcomeFromRubricTool(raw, f.PayloadHash)
		if err != nil {
			return nil, nil, err
		}
		if af != nil {
			agentFindings = append(agentFindings, *af)
		}
		if d != nil {
			diagnostics = append(diagnostics, *d)
		}
	}
	return agentFindings, diagnostics, nil
}

func hiddenMutationSignal(payload json.RawMessage) (codesignal.Signal, bool) {
	var sig codesignal.Signal
	if err := json.Unmarshal(payload, &sig); err != nil {
		return codesignal.Signal{}, false
	}
	if sig.Kind != "hidden_input_mutation" && sig.RuleID != "state.hidden_input_mutation" {
		return codesignal.Signal{}, false
	}
	return sig, true
}

func judgeChangeCohesion(ctx context.Context, loop *agentloop.Loop, fileMetas []rubrics.FileMeta, detFindings []JobFinding) ([]JobFinding, []JobDiagnostic, error) {
	detPayloads := make([]json.RawMessage, 0, len(detFindings))
	for _, f := range detFindings {
		if f.Source == FindingSourceDeterministic {
			detPayloads = append(detPayloads, f.Payload)
		}
	}
	findingsJSON := json.RawMessage("[]")
	if len(detPayloads) > 0 {
		var err error
		findingsJSON, err = json.Marshal(detPayloads)
		if err != nil {
			return nil, nil, err
		}
	}
	cohesionArgs, err := json.Marshal(map[string]any{
		"findings": json.RawMessage(findingsJSON),
		"files":    fileMetas,
	})
	if err != nil {
		return nil, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDChangeCohesion, cohesionArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDChangeCohesion, err)
	}
	af, d, err := jobOutcomeFromRubricTool(raw)
	if err != nil {
		return nil, nil, err
	}
	var agentFindings []JobFinding
	var diagnostics []JobDiagnostic
	if af != nil {
		agentFindings = append(agentFindings, *af)
	}
	if d != nil {
		diagnostics = append(diagnostics, *d)
	}
	return agentFindings, diagnostics, nil
}
