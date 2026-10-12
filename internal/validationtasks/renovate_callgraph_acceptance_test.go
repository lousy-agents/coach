package validationtasks

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("Renovate call-graph provenance updates", func() {
	When("golang.org/x/tools is upgraded", func() {
		It("extracts the public provenance pin and groups it with the Go module update", func() {
			var config struct {
				CustomManagers []struct {
					CustomType          string
					ManagerFilePatterns []string
					MatchStrings        []string
					DepNameTemplate     string
					DatasourceTemplate  string
				}
				PackageRules []struct {
					MatchManagers     []string
					MatchPackageNames []string
					GroupName         string
				}
			}
			data, err := os.ReadFile("../../renovate.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(data, &config)).To(Succeed())
			source, err := os.ReadFile("../../pkg/projectmodel/go_callgraph.go")
			Expect(err).NotTo(HaveOccurred())

			var extracted bool
			for _, manager := range config.CustomManagers {
				if manager.DepNameTemplate != "golang.org/x/tools" {
					continue
				}
				Expect(manager.CustomType).To(Equal("regex"))
				Expect(manager.DatasourceTemplate).To(Equal("go"))
				Expect(manager.ManagerFilePatterns).To(HaveLen(1))
				filePattern, err := regexp.Compile(strings.TrimSuffix(strings.TrimPrefix(manager.ManagerFilePatterns[0], "/"), "/"))
				Expect(err).NotTo(HaveOccurred())
				Expect(filePattern.MatchString("pkg/projectmodel/go_callgraph.go")).To(BeTrue())
				Expect(filePattern.MatchString("pkg/projectmodel/go_callgraph_test.go")).To(BeFalse())
				Expect(manager.MatchStrings).To(HaveLen(1))
				pattern, err := regexp.Compile(manager.MatchStrings[0])
				Expect(err).NotTo(HaveOccurred())
				matches := pattern.FindAllStringSubmatchIndex(string(source), -1)
				Expect(matches).To(HaveLen(1), "extract exactly the constant, not comments or unrelated versions")
				group := pattern.SubexpIndex("currentValue")
				Expect(group).To(BeNumerically(">", 0))
				start, end := matches[0][2*group], matches[0][2*group+1]
				_, pinnedVersion, found := strings.Cut(projectmodel.CallGraphAlgorithm, "golang.org/x/tools@")
				Expect(found).To(BeTrue())
				Expect(string(source[start:end])).To(Equal(pinnedVersion))
				updated := string(source[:start]) + "v0.52.0" + string(source[end:])
				Expect(updated).To(ContainSubstring(`const CallGraphAlgorithm = "go-callgraph-static@1+golang.org/x/tools@v0.52.0"`))
				extracted = true
			}
			Expect(extracted).To(BeTrue(), "Renovate must discover the x/tools pin in CallGraphAlgorithm")

			var grouped bool
			for _, rule := range config.PackageRules {
				if rule.GroupName != "golang-x" {
					continue
				}
				Expect(rule.MatchManagers).To(ContainElements("gomod", "custom.regex"))
				var matchesTools bool
				for _, selector := range rule.MatchPackageNames {
					pattern, err := regexp.Compile(strings.TrimSuffix(strings.TrimPrefix(selector, "/"), "/"))
					Expect(err).NotTo(HaveOccurred())
					matchesTools = matchesTools || pattern.MatchString("golang.org/x/tools")
				}
				Expect(matchesTools).To(BeTrue(), "both manager entries must select the same golang-x dependency group")
				grouped = true
			}
			Expect(grouped).To(BeTrue(), "the provenance pin and go.mod update must share one PR")
		})
	})
})
