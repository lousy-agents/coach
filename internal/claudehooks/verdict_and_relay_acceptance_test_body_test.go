package claudehooks

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_verdictAndRelayAcceptanceTest_89(subagentType, prompt string) bool {
	GinkgoHelper()
	out := runHookPayload("verify-context-relay.sh", map[string]any{
		"tool_input": map[string]string{"subagent_type": subagentType, "prompt": prompt},
	})
	if strings.TrimSpace(out) == "" {
		return false
	}
	var payload struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	Expect(json.Unmarshal([]byte(out), &payload)).To(Succeed())
	return payload.HookSpecificOutput.PermissionDecision == "deny"
}
