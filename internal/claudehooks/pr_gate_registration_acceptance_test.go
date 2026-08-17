package claudehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gateRegistration is one PreToolUse entry routed to gate-pr-creation.sh. The
// `if` clause matters as much as the matcher: after the push gate landed there
// are two Bash registrations, distinguishable only by their `if`. A spec that
// collected matchers alone would report "Bash is covered" while the push
// registration was missing -- green for the wrong reason, which is how this
// repository has shipped an inert registration before.
type gateRegistration struct {
	Matcher string
	If      string
}

func gateRegistrations() []gateRegistration {
	GinkgoHelper()

	raw, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	Expect(err).NotTo(HaveOccurred())

	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					If   string   `json:"if"`
					Args []string `json:"args"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	Expect(json.Unmarshal(raw, &settings)).To(Succeed())

	var out []gateRegistration
	for _, reg := range settings.Hooks.PreToolUse {
		for _, h := range reg.Hooks {
			if strings.Contains(strings.Join(h.Args, " "), "gate-pr-creation.sh") {
				out = append(out, gateRegistration{Matcher: reg.Matcher, If: h.If})
			}
		}
	}
	return out
}

// Which publishing paths exist depends on the surface, and none of these
// registrations is redundant. Locally the repository supplies no GitHub MCP
// server at all (.mcp.json declares context7, lousy-agents and
// sequential-thinking only), so publishing goes through the shell. In CCR `gh`
// is absent -- exit 127 -- and PR creation goes through the harness-supplied
// MCP server. Deleting any one of them fails open on some surface while every
// other check stays green.
var _ = Describe("publish gate registration", func() {
	var regs []gateRegistration

	BeforeEach(func() {
		regs = gateRegistrations()
		Expect(regs).NotTo(BeEmpty(), "nothing routes to the gate; every assertion below would be vacuous")
	})

	hasBashIf := func(fragment string) bool {
		for _, r := range regs {
			if r.Matcher == "Bash" && strings.Contains(r.If, fragment) {
				return true
			}
		}
		return false
	}

	hasMatcher := func(name string) bool {
		for _, r := range regs {
			if r.Matcher == name {
				return true
			}
		}
		return false
	}

	When("a pull request is opened from a shell", func() {
		It("is gated", func() {
			Expect(hasBashIf("gh pr create")).To(BeTrue(),
				"local sessions have no GitHub MCP server; losing this leaves `gh pr create` ungated there")
		})
	})

	// The push is where tree identity is actually decided. The PR body describes
	// a tree the gate inspected; without this the branch can carry another one,
	// and every repair push made while driving a red PR to green is unchecked.
	When("commits are pushed to the remote", func() {
		It("is gated on both surfaces", func() {
			Expect(hasBashIf("git push")).To(BeTrue(),
				"pushes are Bash on every surface; this is the only registration covering the repair path")
		})
	})

	When("the remote is reached through the GitHub API instead of a shell", func() {
		It("gates every tool that can write to it", func() {
			for _, tool := range []string{
				"mcp__github__create_pull_request",
				"mcp__github__push_files",
				"mcp__github__create_or_update_file",
				"mcp__github__delete_file",
			} {
				Expect(hasMatcher(tool)).To(BeTrue(),
					"%s writes to the remote with no shell involved, so the Bash registrations never see it", tool)
			}
		})
	})
})
