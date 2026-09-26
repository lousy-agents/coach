package codesignal_test

import (
	"encoding/json"
	"os"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("Project provenance, scope, and next-actions frozen types (AC-EVD-1, AC-SHAPE)", func() {
	When("Report's json struct tags are inspected", func() {
		It("includes project_provenance, project_scope, and project_next_actions as json tags on Report", func() {
			tags := reportJSONTagNames(reflect.TypeOf(codesignal.Report{}))
			Expect(tags).To(ContainElement("project_provenance"))
			Expect(tags).To(ContainElement("project_scope"))
			Expect(tags).To(ContainElement("project_next_actions"))
		})
	})

	When("a JSON payload with project_provenance is unmarshaled into Report and remarshaled", func() {
		It("preserves project_provenance in the output (field must exist with omitempty-pointer semantics)", func() {
			designJSON := `{
				"schema_version": "2",
				"project_provenance": {
					"language": "typescript",
					"config_digest": "pcfg_abc",
					"selected_roots": ["."]
				}
			}`
			var r codesignal.Report
			Expect(json.Unmarshal([]byte(designJSON), &r)).To(Succeed())
			fields := rawReportFields(&r)
			Expect(fields).To(HaveKey("project_provenance"))
		})
	})

	When("a schema-2 TypeScript report has all three provenance fields populated", func() {
		var fields map[string]json.RawMessage

		BeforeEach(func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectProvenance: &codesignal.ProjectProvenance{
					Language:      "typescript",
					ConfigDigest:  "pcfg_test",
					SelectedRoots: []string{"."},
					Analyzer: codesignal.ProvenanceAnalyzer{
						Version:         "v1.0.0",
						Digest:          "sha256:abc",
						ProtocolVersion: 1,
					},
					Runtime: codesignal.ProvenanceRuntime{
						Kind:            "node",
						Version:         "20.0.0",
						Origin:          "path",
						CompilerVersion: "5.4.0",
						CompilerOrigin:  "project",
					},
					PackageManager: &codesignal.ProvenancePackageManager{
						Kind:    "npm",
						Version: "10.0.0",
						Origin:  "lockfile",
					},
					Head: codesignal.ProvenanceRevision{
						Revision: "HEAD_SHA",
						Coverage: codesignal.ProvenanceCoverage{
							Model:        "complete",
							Bypass:       "not_requested",
							Reachability: "incomplete",
						},
					},
				},
			}
			fields = rawReportFields(report)
		})

		It("emits project_provenance with the frozen Design field names", func() {
			Expect(fields).To(HaveKey("project_provenance"))
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			Expect(prov).To(HaveKey("language"))
			Expect(prov).To(HaveKey("config_digest"))
			Expect(prov).To(HaveKey("selected_roots"))
			Expect(prov).To(HaveKey("analyzer"))
			Expect(prov).To(HaveKey("runtime"))
			Expect(prov).To(HaveKey("package_manager"))
			Expect(prov).To(HaveKey("head"))
		})

		It("emits head.coverage with model, bypass, and reachability", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			var head map[string]json.RawMessage
			Expect(json.Unmarshal(prov["head"], &head)).To(Succeed())
			Expect(head).To(HaveKey("revision"))
			Expect(head).To(HaveKey("coverage"))
			var coverage map[string]json.RawMessage
			Expect(json.Unmarshal(head["coverage"], &coverage)).To(Succeed())
			Expect(coverage).To(HaveKey("model"))
			Expect(coverage).To(HaveKey("bypass"))
			Expect(coverage).To(HaveKey("reachability"))
		})

		It("emits the correct coverage values", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			var head map[string]json.RawMessage
			Expect(json.Unmarshal(prov["head"], &head)).To(Succeed())
			var cov map[string]json.RawMessage
			Expect(json.Unmarshal(head["coverage"], &cov)).To(Succeed())
			var model, bypass, reachability string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(json.Unmarshal(cov["bypass"], &bypass)).To(Succeed())
			Expect(json.Unmarshal(cov["reachability"], &reachability)).To(Succeed())
			Expect(model).To(Equal("complete"))
			Expect(bypass).To(Equal("not_requested"))
			Expect(reachability).To(Equal("incomplete"))
		})

		It("emits analyzer with version, digest, and protocol_version", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			var analyzer map[string]json.RawMessage
			Expect(json.Unmarshal(prov["analyzer"], &analyzer)).To(Succeed())
			Expect(analyzer).To(HaveKey("version"))
			Expect(analyzer).To(HaveKey("digest"))
			Expect(analyzer).To(HaveKey("protocol_version"))
		})

		It("emits runtime with kind, version, origin, compiler_version, compiler_origin", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			var runtime map[string]json.RawMessage
			Expect(json.Unmarshal(prov["runtime"], &runtime)).To(Succeed())
			Expect(runtime).To(HaveKey("kind"))
			Expect(runtime).To(HaveKey("version"))
			Expect(runtime).To(HaveKey("origin"))
			Expect(runtime).To(HaveKey("compiler_version"))
			Expect(runtime).To(HaveKey("compiler_origin"))
			Expect(runtime).NotTo(HaveKey("declared_version"), "declared_version must be omitted when empty")
		})

		It("emits package_manager with kind, version, and origin", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			var pm map[string]json.RawMessage
			Expect(json.Unmarshal(prov["package_manager"], &pm)).To(Succeed())
			Expect(pm).To(HaveKey("kind"))
			Expect(pm).To(HaveKey("version"))
			Expect(pm).To(HaveKey("origin"))
		})

		It("omits base when not set", func() {
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			Expect(prov).NotTo(HaveKey("base"))
		})
	})

	When("a schema-2 Go report has nil provenance fields", func() {
		It("omits project_provenance, project_scope, and project_next_actions while retaining project_changes", func() {
			report := &codesignal.Report{SchemaVersion: "2"}
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_provenance"))
			Expect(fields).NotTo(HaveKey("project_scope"))
			Expect(fields).NotTo(HaveKey("project_next_actions"))
			Expect(fields).To(HaveKey("project_changes"))
		})
	})

	When("project_scope is populated with head revision data", func() {
		It("emits project_scope with inclusion_rule, pattern_set, and head revision fields", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectScope: &codesignal.ProjectScopeReport{
					InclusionRule: "tsconfig_includes_no_test_classification",
					PatternSet:    "ts-source-sink-registry@1",
					Head: codesignal.ProjectScopeRevisionReport{
						Revision: "HEAD_SHA",
						Roots: []projectmodel.ProjectScopeRoot{
							{Root: ".", CandidateFiles: 42, AnalyzedFiles: 42},
						},
						MatchedLayers:   []string{"handlers", "db"},
						UnmatchedLayers: []string{"queue"},
					},
				},
			}
			fields := rawReportFields(report)
			Expect(fields).To(HaveKey("project_scope"))

			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())
			Expect(scope).To(HaveKey("inclusion_rule"))
			Expect(scope).To(HaveKey("pattern_set"))
			Expect(scope).To(HaveKey("head"))
			Expect(scope).NotTo(HaveKey("base"))

			var incRule, patternSet string
			Expect(json.Unmarshal(scope["inclusion_rule"], &incRule)).To(Succeed())
			Expect(json.Unmarshal(scope["pattern_set"], &patternSet)).To(Succeed())
			Expect(incRule).To(Equal("tsconfig_includes_no_test_classification"))
			Expect(patternSet).To(Equal("ts-source-sink-registry@1"))

			var head map[string]json.RawMessage
			Expect(json.Unmarshal(scope["head"], &head)).To(Succeed())
			Expect(head).To(HaveKey("revision"))
			Expect(head).To(HaveKey("roots"))
			Expect(head).To(HaveKey("matched_layers"))
			Expect(head).To(HaveKey("unmatched_layers"))
		})
	})

	When("project_next_actions is populated", func() {
		It("emits project_next_actions as an array of objects with kind", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectNextActions: []codesignal.ProjectNextAction{
					{Kind: "review_policy_coverage"},
					{Kind: "inspect_diagnostics"},
				},
			}
			fields := rawReportFields(report)
			Expect(fields).To(HaveKey("project_next_actions"))
			var actions []map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_next_actions"], &actions)).To(Succeed())
			Expect(actions).To(HaveLen(2))
			var kind string
			Expect(json.Unmarshal(actions[0]["kind"], &kind)).To(Succeed())
			Expect(kind).To(Equal("review_policy_coverage"))
		})

		It("omits project_next_actions when the slice is nil", func() {
			report := &codesignal.Report{SchemaVersion: "2"}
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		})
	})

	When("ProjectProvenance and ProjectScopeRevisionReport have nil slice fields", func() {
		It("marshals selected_roots, roots, matched_layers, and unmatched_layers as [] not null", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectProvenance: &codesignal.ProjectProvenance{
					Language:      "typescript",
					SelectedRoots: nil,
					Analyzer:      codesignal.ProvenanceAnalyzer{ProtocolVersion: 1},
					Runtime:       codesignal.ProvenanceRuntime{Kind: "node"},
					Head:          codesignal.ProvenanceRevision{Revision: "H"},
				},
				ProjectScope: &codesignal.ProjectScopeReport{
					InclusionRule: "x",
					PatternSet:    "y",
					Head: codesignal.ProjectScopeRevisionReport{
						Revision:        "H",
						Roots:           nil,
						MatchedLayers:   nil,
						UnmatchedLayers: nil,
					},
					Base: &codesignal.ProjectScopeRevisionReport{
						Revision:        "B",
						Roots:           nil,
						MatchedLayers:   nil,
						UnmatchedLayers: nil,
					},
				},
			}
			fields := rawReportFields(report)

			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			Expect(string(prov["selected_roots"])).To(Equal("[]"), "selected_roots must marshal as [] not null when nil")

			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())
			var head map[string]json.RawMessage
			Expect(json.Unmarshal(scope["head"], &head)).To(Succeed())
			Expect(string(head["roots"])).To(Equal("[]"), "roots must marshal as [] not null when nil")
			Expect(string(head["matched_layers"])).To(Equal("[]"), "matched_layers must marshal as [] not null when nil")
			Expect(string(head["unmatched_layers"])).To(Equal("[]"), "unmatched_layers must marshal as [] not null when nil")

			var base map[string]json.RawMessage
			Expect(json.Unmarshal(scope["base"], &base)).To(Succeed())
			Expect(string(base["roots"])).To(Equal("[]"), "base.roots must marshal as [] not null when nil")
			Expect(string(base["matched_layers"])).To(Equal("[]"), "base.matched_layers must marshal as [] not null when nil")
			Expect(string(base["unmatched_layers"])).To(Equal("[]"), "base.unmatched_layers must marshal as [] not null when nil")
		})

		It("emits the frozen wire keys for a populated ProjectScope.Base and ProjectProvenance.Base (diff-mode shape)", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectProvenance: &codesignal.ProjectProvenance{
					Language:      "typescript",
					ConfigDigest:  "pcfg_diff",
					SelectedRoots: []string{"."},
					Analyzer:      codesignal.ProvenanceAnalyzer{Version: "v1.0.0", Digest: "sha256:abc", ProtocolVersion: 1},
					Runtime:       codesignal.ProvenanceRuntime{Kind: "node", Version: "20.0.0", Origin: "path", CompilerVersion: "5.4.0", CompilerOrigin: "project"},
					Head:          codesignal.ProvenanceRevision{Revision: "HEAD_SHA", Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested", Reachability: "complete"}},
					Base:          &codesignal.ProvenanceRevision{Revision: "BASE_SHA", Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested", Reachability: "not_run"}},
				},
				ProjectScope: &codesignal.ProjectScopeReport{
					InclusionRule: "tsconfig_includes_no_test_classification",
					PatternSet:    "ts-source-sink-registry@1",
					Head: codesignal.ProjectScopeRevisionReport{
						Revision:        "HEAD_SHA",
						Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 10, AnalyzedFiles: 10}},
						MatchedLayers:   []string{"handlers"},
						UnmatchedLayers: []string{},
					},
					Base: &codesignal.ProjectScopeRevisionReport{
						Revision:        "BASE_SHA",
						Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 8, AnalyzedFiles: 8}},
						MatchedLayers:   []string{"handlers"},
						UnmatchedLayers: []string{},
					},
				},
			}
			fields := rawReportFields(report)

			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			Expect(prov).To(HaveKey("base"), "project_provenance.base must be present when set")
			var provBase map[string]json.RawMessage
			Expect(json.Unmarshal(prov["base"], &provBase)).To(Succeed())
			Expect(provBase).To(HaveKey("revision"))
			Expect(provBase).To(HaveKey("coverage"))
			var provBaseCoverage map[string]json.RawMessage
			Expect(json.Unmarshal(provBase["coverage"], &provBaseCoverage)).To(Succeed())
			Expect(provBaseCoverage).To(HaveKey("model"))
			Expect(provBaseCoverage).To(HaveKey("bypass"))
			Expect(provBaseCoverage).To(HaveKey("reachability"))

			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())
			Expect(scope).To(HaveKey("base"), "project_scope.base must be present when set")
			var scopeBase map[string]json.RawMessage
			Expect(json.Unmarshal(scope["base"], &scopeBase)).To(Succeed())
			Expect(scopeBase).To(HaveKey("revision"))
			Expect(scopeBase).To(HaveKey("roots"))
			Expect(scopeBase).To(HaveKey("matched_layers"))
			Expect(scopeBase).To(HaveKey("unmatched_layers"))

			var revision string
			Expect(json.Unmarshal(scopeBase["revision"], &revision)).To(Succeed())
			Expect(revision).To(Equal("BASE_SHA"))
		})
	})

	When("ProjectProvenance has an empty config_digest", func() {
		It("still emits config_digest in the marshaled output (frozen schema key, not omitempty)", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				ProjectProvenance: &codesignal.ProjectProvenance{
					Language:      "typescript",
					ConfigDigest:  "",
					SelectedRoots: []string{},
					Analyzer:      codesignal.ProvenanceAnalyzer{ProtocolVersion: 1},
					Runtime:       codesignal.ProvenanceRuntime{Kind: "node"},
					Head:          codesignal.ProvenanceRevision{Revision: "H"},
				},
			}
			fields := rawReportFields(report)
			var prov map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
			Expect(prov).To(HaveKey("config_digest"), "config_digest must be present even when empty")
		})
	})

	When("the minimal golden file is loaded", func() {
		It("includes summary.unknown_signals (locking PR #414)", func() {
			data, err := os.ReadFile("testdata/golden/minimal_report.json")
			Expect(err).NotTo(HaveOccurred())
			var root map[string]json.RawMessage
			Expect(json.Unmarshal(data, &root)).To(Succeed())
			Expect(root).To(HaveKey("summary"))
			var summary map[string]json.RawMessage
			Expect(json.Unmarshal(root["summary"], &summary)).To(Succeed())
			Expect(summary).To(HaveKey("unknown_signals"))
		})
	})
})
