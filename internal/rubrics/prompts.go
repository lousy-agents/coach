package rubrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

const hiddenMutationSystemPrompt = `You are a code-quality judge for the Coach platform.
Evaluate one deterministic hidden_input_mutation finding in baseline file context.
Question: Does the mutation hide input state in a way that will surprise a reviewer or complicate future changes?
Distinguish benign constructor wiring from hidden state mutation.
Respond only with JSON matching the provided output schema.
Do not modify, suppress, or restate deterministic findings as if they were yours to alter.`

const changeCohesionSystemPrompt = `You are a code-quality judge for the Coach platform.
Evaluate whether deterministic findings and analyzed files form a cohesive unit of concern.
Question (baseline): Do the analyzed files and findings cluster into coherent areas of concern, or is structural risk scattered across unrelated packages/modules?
Flag tangled imports and cross-file coupling spikes in the snapshot.
Respond only with JSON matching the provided output schema.
Do not modify, suppress, or restate deterministic findings as if they were yours to alter.`

// AssembleHiddenMutationMessages builds gateway Messages attaching one
// deterministic hidden_input_mutation finding plus baseline file context.
func AssembleHiddenMutationMessages(ev HiddenMutationEvidence) []modelgateway.Message {
	var b strings.Builder
	b.WriteString("## Deterministic finding (hidden_input_mutation)\n")
	b.WriteString(formatJSONEvidence(ev.Finding))
	b.WriteString("\n\n## Baseline file context\n")
	b.WriteString(fmt.Sprintf("path: %s\n", ev.File.Path))
	b.WriteString(fmt.Sprintf("language: %s\n", ev.File.Language))
	if ev.File.Content != "" {
		b.WriteString("content:\n```\n")
		b.WriteString(ev.File.Content)
		if !strings.HasSuffix(ev.File.Content, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return []modelgateway.Message{
		{Role: "system", Content: hiddenMutationSystemPrompt},
		{Role: "user", Content: b.String()},
	}
}

// AssembleChangeCohesionMessages builds gateway Messages attaching the full
// set of deterministic findings plus file metadata.
func AssembleChangeCohesionMessages(ev ChangeCohesionEvidence) []modelgateway.Message {
	var b strings.Builder
	b.WriteString("## Deterministic findings\n")
	b.WriteString(formatJSONEvidence(ev.Findings))
	b.WriteString("\n\n## File metadata\n")
	if len(ev.Files) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, f := range ev.Files {
			b.WriteString(fmt.Sprintf("- path: %s, language: %s\n", f.Path, f.Language))
		}
	}
	return []modelgateway.Message{
		{Role: "system", Content: changeCohesionSystemPrompt},
		{Role: "user", Content: b.String()},
	}
}

func formatJSONEvidence(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
