package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// judgmentBudgetExceededError is returned when the judgment-phase wall/tool
// budget stops the pack loop mid-phase. Agent findings from completed packs
// are already persisted; completeAfterJudgmentError records the diagnostic.
type judgmentBudgetExceededError struct {
	Judged    int
	Remaining int
	Err       error
}

func (e *judgmentBudgetExceededError) Error() string {
	return fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d: %v", e.Judged, e.Remaining, e.Err)
}

func (e *judgmentBudgetExceededError) Unwrap() error { return e.Err }

func judgeBaselineViaLoop(
	ctx context.Context,
	loop *agentloop.Loop,
	files []loadedBaselineFile,
	detFindings []JobFinding,
	w BaselineJobWriter,
	packCfg rubrics.PackConfig,
	maxHiddenMutationJudgments int,
) ([]JobFinding, []JobDiagnostic, error) {
	byPath := make(map[string]loadedBaselineFile, len(files))
	fileMetas := make([]rubrics.FileMeta, 0, len(files))
	for _, f := range files {
		byPath[f.Path] = f
		fileMetas = append(fileMetas, rubrics.FileMeta{
			Path:     f.Path,
			Language: string(f.Language),
		})
	}

	packCfg = rubrics.ApplyPackConfigDefaults(packCfg)
	maxJudgments := resolveMaxHiddenMutationJudgments(maxHiddenMutationJudgments)

	agentFindings, diagnostics, err := judgeHiddenMutationFindings(ctx, loop, byPath, detFindings, w, packCfg, maxJudgments)
	if err != nil {
		return agentFindings, diagnostics, err
	}

	cohesionFindings, cohesionDiags, err := judgeChangeCohesion(ctx, loop, fileMetas, detFindings)
	if err != nil {
		if errors.Is(err, agentloop.ErrBudgetExceeded) {
			// HM packs finished; wall died on cohesion. Report HM agent rows as judged.
			return agentFindings, diagnostics, &judgmentBudgetExceededError{
				Judged:    len(agentFindings),
				Remaining: 0,
				Err:       err,
			}
		}
		return agentFindings, diagnostics, err
	}
	if err := insertBaselineFindings(ctx, w, cohesionFindings); err != nil {
		return agentFindings, diagnostics, err
	}
	agentFindings = append(agentFindings, cohesionFindings...)
	diagnostics = append(diagnostics, cohesionDiags...)
	return agentFindings, diagnostics, nil
}

func judgeHiddenMutationFindings(
	ctx context.Context,
	loop *agentloop.Loop,
	byPath map[string]loadedBaselineFile,
	detFindings []JobFinding,
	w BaselineJobWriter,
	packCfg rubrics.PackConfig,
	maxJudgments int,
) ([]JobFinding, []JobDiagnostic, error) {
	type candMeta struct {
		finding JobFinding
		sig     codesignal.Signal
	}

	var (
		cands []rubrics.PackCandidate
		metas []candMeta
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
		startRow := int(sig.Location.StartRow)
		window := rubrics.FormatSpanWindow(lf.Content, startRow, packCfg.EvidenceWindowLines)
		cands = append(cands, rubrics.PackCandidate{
			FindingRef:    f.PayloadHash,
			Path:          sig.Path,
			StartRow:      startRow,
			Severity:      string(sig.Severity),
			Confidence:    string(sig.Confidence),
			PayloadJSON:   append([]byte(nil), f.Payload...),
			EvidenceChars: len(window),
		})
		metas = append(metas, candMeta{finding: f, sig: sig})
	}
	if len(cands) == 0 {
		return nil, nil, nil
	}

	byRef := make(map[string]candMeta, len(metas))
	for i, c := range cands {
		byRef[c.FindingRef] = metas[i]
	}

	// Priority cap before packing (Story 3): select subset, then pack.
	var diagnostics []JobDiagnostic
	selected, omitted := PrioritizeJudgmentCandidates(cands, maxJudgments)
	if omitted > 0 {
		diagnostics = append(diagnostics, judgmentCapDiagnostic(len(selected), omitted))
	}
	cands = selected

	packs := rubrics.PackJudgmentCandidates(cands, packCfg)
	total := len(cands)
	var (
		agentFindings []JobFinding
		judged        int
	)

	for _, pack := range packs {
		items := make([]rubrics.HiddenMutationPackItem, 0, len(pack.FindingRefs))
		for _, ref := range pack.FindingRefs {
			meta, ok := byRef[ref]
			if !ok {
				continue
			}
			lf, found := byPath[meta.sig.Path]
			if !found {
				lf = loadedBaselineFile{Path: meta.sig.Path}
			}
			window := rubrics.FormatSpanWindow(lf.Content, int(meta.sig.Location.StartRow), packCfg.EvidenceWindowLines)
			items = append(items, rubrics.HiddenMutationPackItem{
				FindingRef: ref,
				Finding:    append(json.RawMessage(nil), meta.finding.Payload...),
				File: rubrics.FileContext{
					Path:     lf.Path,
					Language: string(lf.Language),
					Content:  window,
				},
			})
		}
		if len(items) == 0 {
			continue
		}

		args, err := json.Marshal(map[string]any{"items": items})
		if err != nil {
			return agentFindings, diagnostics, err
		}
		raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDHiddenMutationContextualization, args)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return agentFindings, diagnostics, err
			}
			if errors.Is(err, agentloop.ErrBudgetExceeded) {
				remaining := total - judged
				return agentFindings, diagnostics, &judgmentBudgetExceededError{
					Judged:    judged,
					Remaining: remaining,
					Err:       err,
				}
			}
			return agentFindings, diagnostics, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDHiddenMutationContextualization, err)
		}

		packFindings, packDiags, err := jobOutcomesFromHiddenMutationResult(raw)
		if err != nil {
			return agentFindings, diagnostics, err
		}
		// Incremental persist so budget exceed keeps completed packs.
		if err := insertBaselineFindings(ctx, w, packFindings); err != nil {
			return agentFindings, diagnostics, err
		}
		agentFindings = append(agentFindings, packFindings...)
		diagnostics = append(diagnostics, packDiags...)
		// Count successful agent rows only — diagnostics-only packs must not
		// inflate judged= in the Story 2 budget diagnostic.
		judged += len(packFindings)
	}
	return agentFindings, diagnostics, nil
}

// jobOutcomesFromHiddenMutationResult maps a singular ToolResult or pack
// {"results":[...]} envelope to job findings/diagnostics. Pack items use
// FindingRef (deterministic PayloadHash) as the payload_hash discriminator.
func jobOutcomesFromHiddenMutationResult(raw json.RawMessage) ([]JobFinding, []JobDiagnostic, error) {
	if rubrics.IsToolPackResult(raw) {
		pack, err := rubrics.ParseToolPackResult(raw)
		if err != nil {
			return nil, nil, err
		}
		var findings []JobFinding
		var diags []JobDiagnostic
		for _, tr := range pack.Results {
			itemRaw, err := json.Marshal(tr)
			if err != nil {
				return nil, nil, err
			}
			disc := tr.FindingRef
			af, d, err := jobOutcomeFromRubricTool(itemRaw, disc)
			if err != nil {
				return nil, nil, err
			}
			if af != nil {
				findings = append(findings, *af)
			}
			if d != nil {
				diags = append(diags, *d)
			}
		}
		return findings, diags, nil
	}

	// Singular envelope (one-item pack may take the singular tool path).
	var tr rubrics.ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding rubric tool result: %w", err)
	}
	var disc []string
	if tr.FindingRef != "" {
		disc = []string{tr.FindingRef}
	}
	af, d, err := jobOutcomeFromRubricTool(raw, disc...)
	if err != nil {
		return nil, nil, err
	}
	var findings []JobFinding
	var diags []JobDiagnostic
	if af != nil {
		findings = append(findings, *af)
	}
	if d != nil {
		diags = append(diags, *d)
	}
	return findings, diags, nil
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

// judgmentBudgetDiagnostic builds the stable diagnostic for mid-phase wall/tool budget stop.
func judgmentBudgetDiagnostic(judged, remaining int, err error) JobDiagnostic {
	msg := fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d", judged, remaining)
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	return JobDiagnostic{
		ID:      watermill.NewUUID(),
		Scope:   "judgment_budget",
		Message: msg,
	}
}
