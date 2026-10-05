package rubrics

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

func validateBatchItem(it batchItemJudgment) error {
	switch it.Judgment {
	case "concern", "acceptable", "unclear":
	default:
		return fmt.Errorf("schema validation failed: judgment value not in enum")
	}
	switch it.Confidence {
	case "high", "medium", "low":
	default:
		return fmt.Errorf("schema validation failed: confidence value not in enum")
	}
	if strings.TrimSpace(it.Rationale) == "" {
		return fmt.Errorf("schema validation failed: rationale must be a non-empty string")
	}
	if len(it.SuggestedFocus) == 0 {
		return fmt.Errorf("schema validation failed: missing required property: suggested_focus")
	}

	if string(it.SuggestedFocus) != "null" {
		var s string
		if err := json.Unmarshal(it.SuggestedFocus, &s); err != nil {
			return fmt.Errorf("schema validation failed: suggested_focus must be string or null")
		}
	}
	return nil
}

// AssembleHiddenMutationPackMessages builds gateway Messages for a multi-finding
// pack with short-rationale guidance and per-item span-local evidence.
func AssembleHiddenMutationPackMessages(ev HiddenMutationPackEvidence) []modelgateway.Message {
	var b strings.Builder
	b.WriteString("## Hidden-mutation judgment pack\n")
	b.WriteString("Judge each item independently. Map every input finding_ref to exactly one output item.\n")
	b.WriteString(shortRationaleGuidance)
	b.WriteByte('\n')
	for _, it := range ev.Items {
		writeHiddenMutationItem(&b, it)
	}
	return []modelgateway.Message{
		{Role: "system", Content: hiddenMutationPackSystemPrompt},
		{Role: "user", Content: b.String()},
	}
}

func writeHiddenMutationItem(b *strings.Builder, it HiddenMutationPackItem) {
	b.WriteString("\n### Item\n")
	b.WriteString("finding_ref: ")
	b.WriteString(it.FindingRef)
	b.WriteByte('\n')
	b.WriteString("## Deterministic finding (hidden_input_mutation)\n")
	b.WriteString(formatJSONEvidence(it.Finding))
	b.WriteString("\n\n## Baseline file context\n")
	b.WriteString(fmt.Sprintf("path: %s\n", it.File.Path))
	b.WriteString(fmt.Sprintf("language: %s\n", it.File.Language))
	if it.File.Content != "" {
		b.WriteString("content (span window):\n```\n")
		b.WriteString(it.File.Content)
		if !strings.HasSuffix(it.File.Content, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
}
