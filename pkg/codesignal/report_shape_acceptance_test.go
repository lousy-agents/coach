package codesignal_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// reportJSONTagNames returns the json tag names declared on t, in
// declaration order, recursing into embedded fields. Used to derive the
// expected key set directly from Report's own struct tags rather than a
// hand-maintained list, so a field added to Report is caught here without
// this test needing an update.
func reportJSONTagNames(t reflect.Type) []string {
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			names = append(names, reportJSONTagNames(f.Type)...)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func rawReportKeys(report *codesignal.Report) []string {
	fields := rawReportFields(report)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	return keys
}

func rawReportFields(report *codesignal.Report) map[string]json.RawMessage {
	raw, err := json.Marshal(report)
	Expect(err).NotTo(HaveOccurred())
	var fields map[string]json.RawMessage
	Expect(json.Unmarshal(raw, &fields)).To(Succeed())
	return fields
}

func rawCoverageFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	Expect(fields).To(HaveKey("coverage"))
	var coverage map[string]json.RawMessage
	Expect(json.Unmarshal(fields["coverage"], &coverage)).To(Succeed())
	return coverage
}

func rawSignalKeys(raw json.RawMessage) []string {
	var m map[string]json.RawMessage
	Expect(json.Unmarshal(raw, &m)).To(Succeed())
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var _ = Describe("Report shape: always-present top-level keys, Coverage members, and Signal key sets (issue #269)", func() {
	When("a schema-1 diff-mode Input has zero files, diagnostics, and coverage exclusions", func() {
		It("still serializes schema_version/scope/summary/signals/diagnostics/coverage as present, non-null keys", func() {
			report := build(codesignal.Options{}, codesignal.Input{})
			fields := rawReportFields(report)

			Expect(fields).To(HaveKey("schema_version"))
			Expect(fields).To(HaveKey("scope"))
			Expect(fields).To(HaveKey("summary"))

			Expect(fields).To(HaveKey("signals"))
			Expect(string(fields["signals"])).To(Equal("[]"), "signals must never be null when empty")

			Expect(fields).To(HaveKey("diagnostics"))
			Expect(string(fields["diagnostics"])).To(Equal("[]"), "diagnostics must never be null when empty")

			Expect(fields).To(HaveKey("coverage"))
			Expect(string(fields["coverage"])).NotTo(Equal("null"))
			coverage := rawCoverageFields(fields)
			Expect(coverage).To(HaveKey("unsupported"))
			Expect(string(coverage["unsupported"])).To(Equal("[]"))
			Expect(coverage).To(HaveKey("excluded"))
			Expect(string(coverage["excluded"])).To(Equal("[]"))
		})
	})

	When("a schema-1 baseline-mode Input has a non-nil Coverage with nil Unsupported/Excluded slices", func() {
		It("still serializes coverage.unsupported and coverage.excluded as present, empty arrays rather than null", func() {
			report := build(codesignal.Options{Baseline: true}, codesignal.Input{
				Coverage: &codesignal.Coverage{TrackedFilesDiscovered: 3, FilesAnalyzed: 3},
			})
			fields := rawReportFields(report)

			Expect(fields).To(HaveKey("signals"))
			Expect(string(fields["signals"])).To(Equal("[]"))
			Expect(fields).To(HaveKey("diagnostics"))
			Expect(string(fields["diagnostics"])).To(Equal("[]"))

			coverage := rawCoverageFields(fields)
			Expect(string(coverage["unsupported"])).To(Equal("[]"), "unsupported must never be null when the caller's slice is nil")
			Expect(string(coverage["excluded"])).To(Equal("[]"), "excluded must never be null when the caller's slice is nil")
		})
	})

	When("Options.ProjectEnabled is true and the project phase produced no data at all", func() {
		It("still serializes project_changes/project_facts/project_summary/project_coverage present and empty", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{})
			fields := rawReportFields(report)

			var schemaVersion string
			Expect(json.Unmarshal(fields["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("2"))

			Expect(fields).To(HaveKey("project_changes"))
			Expect(string(fields["project_changes"])).To(Equal("[]"))

			Expect(fields).To(HaveKey("project_facts"))
			Expect(string(fields["project_facts"])).To(Equal("[]"))

			Expect(fields).To(HaveKey("project_summary"))
			Expect(string(fields["project_summary"])).NotTo(Equal("null"))

			Expect(fields).To(HaveKey("project_coverage"))
			Expect(string(fields["project_coverage"])).NotTo(Equal("null"))
		})
	})

	When("--project-config is not supplied (Options.ProjectEnabled is false)", func() {
		It("keeps schema_version \"1\" and emits no project_* keys", func() {
			report := build(codesignal.Options{}, codesignal.Input{})
			fields := rawReportFields(report)

			var schemaVersion string
			Expect(json.Unmarshal(fields["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("1"))

			Expect(fields).NotTo(HaveKey("project_changes"))
			Expect(fields).NotTo(HaveKey("project_facts"))
			Expect(fields).NotTo(HaveKey("project_summary"))
			Expect(fields).NotTo(HaveKey("project_coverage"))
		})
	})

	When("Report gains or loses a field", func() {
		var expectedKeys []string

		BeforeEach(func() {
			expectedKeys = reportJSONTagNames(reflect.TypeOf(codesignal.Report{}))
		})

		It("keeps schema_version \"2\"'s marshalled key set in sync with every one of Report's own json tags", func() {
			schema2 := codesignal.Report{SchemaVersion: "2"}
			Expect(rawReportKeys(&schema2)).To(ConsistOf(expectedKeys))
		})

		It("keeps schema_version \"1\"'s marshalled key set in sync with Report's own json tags minus the four project_* keys", func() {
			projectKeys := []string{"project_changes", "project_facts", "project_summary", "project_coverage"}
			expectedSchema1Keys := make([]string, 0, len(expectedKeys))
			for _, name := range expectedKeys {
				isProjectKey := false
				for _, p := range projectKeys {
					if name == p {
						isProjectKey = true
						break
					}
				}
				if !isProjectKey {
					expectedSchema1Keys = append(expectedSchema1Keys, name)
				}
			}

			schema1 := codesignal.Report{SchemaVersion: "1"}
			Expect(rawReportKeys(&schema1)).To(ConsistOf(expectedSchema1Keys))
		})
	})

	When("a report contains both a non-project-origin Signal and a project-origin Signal", func() {
		var rawSignals []json.RawMessage

		BeforeEach(func() {
			change := codesignal.ProjectChange{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "project.import_cycle",
				RuleVersion: "1",
				Kind:        "cycle",
				Category:    codesignal.Category("architecture"),
				Severity:    codesignal.Severity("medium"),
				Confidence:  codesignal.Confidence("high"),
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/a/a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Evidence:   "pkg/a -> pkg/b -> pkg/a",
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}

			finding := mutation("Update", 4)
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				Files: []codesignal.FileChange{{
					Path: "state.go", Status: "modified", Head: cleanResult("state.go", finding),
				}},
				ProjectChanges:  []codesignal.ProjectChange{change},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.Signals).To(HaveLen(2))

			raw, err := json.Marshal(report.Signals)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(raw, &rawSignals)).To(Succeed())
			Expect(rawSignals).To(HaveLen(2))
		})

		findNonProjectSignal := func() map[string]json.RawMessage {
			for _, r := range rawSignals {
				var m map[string]json.RawMessage
				Expect(json.Unmarshal(r, &m)).To(Succeed())
				var subject string
				if raw, ok := m["subject"]; ok {
					Expect(json.Unmarshal(raw, &subject)).To(Succeed())
				}
				if subject == "Update" {
					return m
				}
			}
			return nil
		}

		It("carries the identical key set on both entries regardless of origin", func() {
			Expect(rawSignalKeys(rawSignals[0])).To(Equal(rawSignalKeys(rawSignals[1])))
		})

		It("marshals every entry with the full always-present key set", func() {
			for _, r := range rawSignals {
				var m map[string]json.RawMessage
				Expect(json.Unmarshal(r, &m)).To(Succeed())

				Expect(m).To(HaveKey("why_it_matters"))
				Expect(m).To(HaveKey("recommendation"))
				Expect(m).To(HaveKey("suggested_skill"))
				Expect(m).To(HaveKey("machine_evidence"))
				Expect(m).To(HaveKey("related_locations"))
				Expect(m).To(HaveKey("path_steps"))
				Expect(m).To(HaveKey("coverage_refs"))
			}
		})

		It("emits empty-typed, not null, evidence fields for a non-project-origin signal", func() {
			nonProjectSignal := findNonProjectSignal()
			Expect(nonProjectSignal).NotTo(BeNil(), "expected to find the file-local mutation signal by its subject")
			Expect(string(nonProjectSignal["machine_evidence"])).To(Equal("{}"), "a non-project-origin signal must emit an empty object, not null")
			Expect(string(nonProjectSignal["related_locations"])).To(Equal("[]"))
			Expect(string(nonProjectSignal["path_steps"])).To(Equal("[]"))
			Expect(string(nonProjectSignal["coverage_refs"])).To(Equal("[]"))
			Expect(string(nonProjectSignal["suggested_skill"])).To(Equal(`""`))
		})
	})
})
