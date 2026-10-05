package projectconfig

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("project-config boundary budgets", func() {
	It("rejects documents that exceed the config size budget before schema decode", func() {
		oversized := []byte(`{"schema_version":"1","roots":["` + strings.Repeat("a", MaxBytes) + `"]}`)
		err := validateJSON(oversized)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("size budget"))
	})

	It("validates a near-budget set of non-overlapping layer prefixes without stalling", func() {
		doc := layerPrefixConfigJSON(512)
		started := time.Now()
		err := validateJSON(doc)
		elapsed := time.Since(started)
		Expect(err).NotTo(HaveOccurred(), "non-overlapping prefixes within budget must validate")
		Expect(elapsed).To(BeNumerically("<", 2*time.Second), "prefix validation must stay sub-quadratic; elapsed=%s", elapsed)
	})

	It("rejects a forbidden_imports entry naming an undeclared layer, rather than silently becoming a permanent no-op edge", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handler","to":"db"}]}`)
		err := validateJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined layer"))
		Expect(err.Error()).To(ContainSubstring("handler"))
	})

	It("rejects overlapping layer prefixes with the stable diagnostic", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"L","prefixes":["services","services/payments"]}]}`)
		err := validateJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer prefixes must be unique and non-overlapping"))
	})

	It("rejects layer prefix counts above the explicit budget", func() {
		err := validateJSON(layerPrefixConfigJSON(MaxLayerPrefixes + 1))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer prefixes exceed budget"))
	})

	It("rejects documents that exceed the JSON nesting budget", func() {
		body_projectAcceptanceTest_rejectsDocumentsThatExceedTheJSONNestingBudget_759()
	})

	It("surfaces a timed-out git child as project_config_invalid", func() {
		originalRunner := runProjectConfigGit
		DeferCleanup(func() { runProjectConfigGit = originalRunner })

		hungGit := func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sleep", "60")
		}
		runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
			return gitrepo.RunBytesBoundedWith(hungGit, dir, MaxBytes, MaxGitStderr, 50*time.Millisecond, args...)
		}

		_, err := Load(".", "HEAD", "project.json")
		Expect(err).To(HaveOccurred())
		var cfgErr *ConfigError
		Expect(err).To(BeAssignableToTypeOf(cfgErr))
		Expect(err.Error()).To(ContainSubstring("project_config_invalid"))
		Expect(err.Error()).To(ContainSubstring("timed out"))
	})

	It("accepts a required_layer that names a declared layer", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"db"}`)
		Expect(validateJSON(doc)).To(Succeed(), "a required_layer naming a declared layer must be a valid config")
	})

	It("treats an empty required_layer the same as omitting the field entirely", func() {
		withEmpty := []byte(`{"schema_version":"1","roots":["."],"required_layer":""}`)
		withoutField := []byte(`{"schema_version":"1","roots":["."]}`)
		Expect(validateJSON(withEmpty)).To(Succeed(), "an empty required_layer must not be treated as naming a (nonexistent) layer")
		Expect(validateJSON(withoutField)).To(Succeed())
	})

	It("rejects a required_layer naming an undeclared layer, mirroring forbidden_imports, rather than silently becoming a permanent no-op", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]}],"required_layer":"database"}`)
		err := validateJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined layer"))
		Expect(err.Error()).To(ContainSubstring("database"))
	})
})

func body_projectAcceptanceTest_rejectsDocumentsThatExceedTheJSONNestingBudget_759() {
	var b strings.Builder
	for i := 0; i < maxJSONDepth+2; i++ {
		b.WriteString(`{"a":`)
	}
	b.WriteString(`1`)
	for i := 0; i < maxJSONDepth+2; i++ {
		b.WriteByte('}')
	}
	err := validateJSON([]byte(b.String()))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("nesting budget"))
}

func layerPrefixConfigJSON(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"schema_version":"1","roots":["."],"layers":[{"name":"L","prefixes":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"p%04d"`, i)
	}
	b.WriteString(`]}]}`)
	return []byte(b.String())
}
