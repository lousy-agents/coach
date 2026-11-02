package validationtasks

import (
	"encoding/json"
	"os"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type renovateConfigForNodeHold struct {
	CustomManagers []struct {
		ManagerFilePatterns []string
		DepNameTemplate     string
	}
	PackageRules []struct {
		MatchManagers               []string
		MatchPackageNames           []string
		MatchUpdateTypes            []string
		RangeStrategy               string
		DependencyDashboardApproval *bool
		Automerge                   *bool
	}
}

func readRenovateConfigForNodeHold() renovateConfigForNodeHold {
	data, err := os.ReadFile("../../renovate.json")
	Expect(err).NotTo(HaveOccurred())
	var config renovateConfigForNodeHold
	Expect(json.Unmarshal(data, &config)).To(Succeed())
	return config
}

var _ = Describe("Renovate holds Node.js major updates for owner approval", func() {
	nodeDependencies := []string{"nodejs/node", "node", "@types/node"}

	When("a new Node.js major is released", func() {
		It("requires dashboard approval and never automerges, for the runtime, the engines range and @types/node alike", func() {
			config := readRenovateConfigForNodeHold()

			for _, dependency := range nodeDependencies {
				var holding int
				for _, rule := range config.PackageRules {
					if !slices.Contains(rule.MatchPackageNames, dependency) || !slices.Contains(rule.MatchUpdateTypes, "major") {
						continue
					}
					holding++
					Expect(rule.DependencyDashboardApproval).NotTo(BeNil(), "%s majors must wait for dashboard approval", dependency)
					Expect(*rule.DependencyDashboardApproval).To(BeTrue(), "%s majors must wait for dashboard approval", dependency)
					Expect(rule.Automerge).NotTo(BeNil(), "%s majors must set automerge explicitly, because the top-level default is true", dependency)
					Expect(*rule.Automerge).To(BeFalse(), "%s majors must not automerge", dependency)
				}
				Expect(holding).To(Equal(1), "exactly one rule should hold %s majors", dependency)
			}
		})

		It("holds only major updates, so minor and patch updates and other dependencies keep flowing", func() {
			config := readRenovateConfigForNodeHold()

			for _, rule := range config.PackageRules {
				if rule.DependencyDashboardApproval == nil {
					continue
				}
				Expect(rule.MatchUpdateTypes).To(Equal([]string{"major"}), "a dashboard-approval rule must be scoped to majors")
				Expect(rule.MatchPackageNames).To(ConsistOf(nodeDependencies), "the hold must not reach dependencies the owner did not name")
			}
		})

		It("names the dependency the mise.toml custom manager actually extracts", func() {
			config := readRenovateConfigForNodeHold()

			var extracted bool
			for _, manager := range config.CustomManagers {
				extracted = extracted || (manager.DepNameTemplate == "nodejs/node" && slices.Contains(manager.ManagerFilePatterns, "/(^|/)mise\\.toml$/"))
			}
			Expect(extracted).To(BeTrue(), "the hold's nodejs/node selector goes stale if the custom manager is renamed")
		})
	})

	When("npm dependencies are updated", func() {
		It("still pins them to exact versions", func() {
			config := readRenovateConfigForNodeHold()

			var pinned bool
			for _, rule := range config.PackageRules {
				if slices.Equal(rule.MatchManagers, []string{"npm"}) && rule.RangeStrategy == "pin" {
					pinned = true
					Expect(rule.MatchPackageNames).To(BeEmpty(), "pinning applies to every npm dependency")
					Expect(rule.MatchUpdateTypes).To(BeEmpty(), "pinning applies to every update type")
				}
			}
			Expect(pinned).To(BeTrue())
		})
	})
})
