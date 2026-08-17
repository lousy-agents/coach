package claudehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The PR gate is registered twice, and the two registrations are not
// redundant: which one can fire depends on the surface. Locally the repository
// supplies no GitHub MCP server at all (.mcp.json declares context7,
// lousy-agents and sequential-thinking only), so PR creation goes through
// `gh pr create` and only the Bash registration is reachable. In CCR `gh` is
// absent -- exit 127 -- and PR creation goes through the harness-supplied
// mcp__github__create_pull_request, so only that registration is reachable.
//
// Each is therefore the sole live PR gate on one surface, and deleting either
// one fails open there while every other check stays green. Nothing detected
// that before this spec: step 0 proves the settings file was *bound*, not that
// a given matcher survived editing it.
var _ = Describe("PR gate registration", func() {
	preToolUseMatchers := func(script string) []string {
		GinkgoHelper()

		raw, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
		Expect(err).NotTo(HaveOccurred())

		var settings struct {
			Hooks struct {
				PreToolUse []struct {
					Matcher string `json:"matcher"`
					Hooks   []struct {
						Args []string `json:"args"`
					} `json:"hooks"`
				} `json:"PreToolUse"`
			} `json:"hooks"`
		}
		Expect(json.Unmarshal(raw, &settings)).To(Succeed())

		var matchers []string
		for _, reg := range settings.Hooks.PreToolUse {
			for _, h := range reg.Hooks {
				if strings.Contains(strings.Join(h.Args, " "), script) {
					matchers = append(matchers, reg.Matcher)
				}
			}
		}
		return matchers
	}

	When("a pull request is opened on either surface", func() {
		It("keeps a registration for the path that surface actually uses", func() {
			matchers := preToolUseMatchers("gate-pr-creation.sh")
			Expect(matchers).To(ContainElement("Bash"),
				"local sessions have no GitHub MCP server; losing this leaves `gh pr create` ungated there")
			Expect(matchers).To(ContainElement("mcp__github__create_pull_request"),
				"CCR has no gh; losing this leaves the primary surface ungated")
		})
	})
})
