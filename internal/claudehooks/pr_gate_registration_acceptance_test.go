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

	hasMatcher := func(name string) bool {
		for _, r := range regs {
			if r.Matcher == name {
				return true
			}
		}
		return false
	}

	// The gate fires on pull-request creation only, and neither registration is
	// redundant: local sessions supply no GitHub MCP server at all (.mcp.json
	// declares context7, lousy-agents and sequential-thinking), while CCR has no
	// gh -- exit 127. Each is the sole live gate on one surface, so deleting
	// either fails open there while every other check stays green.
	//
	// An earlier revision widened this to `git push` and the MCP write tools on
	// an unguarded matcher. It ran on every shell command, refused a commit whose
	// message merely mentioned pushing, and flooded the hook trace -- all to
	// defend a property the required `status` check already provides.
	When("a pull request is opened", func() {
		It("is gated on whichever path the surface actually has", func() {
			var bashIf string
			var bashCount int
			for _, r := range regs {
				if r.Matcher == "Bash" {
					bashCount++
					bashIf = r.If
				}
			}
			Expect(bashCount).To(Equal(1), "one Bash registration, scoped to PR creation")
			Expect(bashIf).To(ContainSubstring("gh pr create"),
				"an unscoped Bash matcher puts this hook on the hot path of every command in the session")
			Expect(hasMatcher("mcp__github__create_pull_request")).To(BeTrue(),
				"CCR has no gh; losing this leaves the primary surface ungated")
		})

		It("does not gate the publishing paths branch protection already covers", func() {
			for _, tool := range []string{
				"mcp__github__push_files",
				"mcp__github__create_or_update_file",
				"mcp__github__delete_file",
			} {
				Expect(hasMatcher(tool)).To(BeFalse(),
					"%s writes reach the remote, but the required status check is what stops them merging; gating here re-buys the cost this narrowing removed", tool)
			}
		})
	})
})
