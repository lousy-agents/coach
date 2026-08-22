package codesignal_test

import (
	"context"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func arbitraryBoundaryParitySnapshotMeta() projectmodel.SnapshotMeta {
	return projectmodel.SnapshotMeta{Revision: "rev", ConfigDigest: "digest-1"}
}

// buildProjectReport drives input through the same Builder.Build call the
// CLI issues for a project-enabled baseline report.
func buildProjectReport(ctx context.Context, input codesignal.Input) *codesignal.Report {
	builder, err := codesignal.New(codesignal.Options{ProjectEnabled: true, Baseline: true})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	report, err := builder.Build(ctx, input)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return report
}

// expectSharedArchitectureShape deliberately does not assert MachineEvidence
// value equality: Go's
// layer_violation/layer_bypass evaluators address importer/importee by
// package directory while TypeScript's address by source file (a real,
// already-shipped asymmetry from issue #215/#216, not a bug this test
// should paper over) -- only the MachineEvidence key set (language aside)
// is required to match.
func expectSharedArchitectureShape(goChange, tsChange codesignal.ProjectChange, ruleID string) {
	ExpectWithOffset(1, goChange.RuleID).To(Equal(ruleID))
	ExpectWithOffset(1, tsChange.RuleID).To(Equal(ruleID))
	ExpectWithOffset(1, tsChange.Kind).To(Equal(goChange.Kind))
	ExpectWithOffset(1, tsChange.Category).To(Equal(goChange.Category))
	ExpectWithOffset(1, tsChange.Severity).To(Equal(goChange.Severity))
	ExpectWithOffset(1, tsChange.Confidence).To(Equal(goChange.Confidence))
	ExpectWithOffset(1, tsChange.RuleVersion).To(Equal(goChange.RuleVersion))
	ExpectWithOffset(1, tsChange.ConfigDigest).To(Equal(goChange.ConfigDigest))
	ExpectWithOffset(1, tsChange.Lifecycle).To(Equal(goChange.Lifecycle))
	ExpectWithOffset(1, tsChange.WhyItMatters).To(Equal(goChange.WhyItMatters))
	ExpectWithOffset(1, tsChange.Recommendation).To(Equal(goChange.Recommendation))
	ExpectWithOffset(1, tsChange.Provenance.Producer).To(Equal(goChange.Provenance.Producer))
	ExpectWithOffset(1, tsChange.Provenance.FindingKind).To(Equal(goChange.Provenance.FindingKind))

	ExpectWithOffset(1, goChange.MachineEvidence).NotTo(HaveKey("language"))
	ExpectWithOffset(1, tsChange.MachineEvidence).To(HaveKeyWithValue("language", "typescript"))
	ExpectWithOffset(1, keysOf(machineEvidenceWithoutLanguage(tsChange.MachineEvidence))).To(ConsistOf(keysOf(goChange.MachineEvidence)), "expected the same MachineEvidence key set on both languages (values may differ: Go addresses by package directory, TS by source file)")
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

var _ = Describe("coach's project-analysis public boundary (codesignal.Builder.Build) across Go and TypeScript backends (issue #218 T1)", func() {
	When("an unambiguous forbidden layer edge exists in both a real Go project model and a TypeScript project model", func() {
		It("emits one baseline architecture.layer_violation ProjectChange on both languages' Reports, identically shaped apart from language provenance", func() {
			ctx := context.Background()

			goSnapshot := fstest.MapFS{
				"go.mod":                   &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
				"pkg/db/db.go":             &fstest.MapFile{Data: []byte("package db\n\nvar Name = \"db\"\n")},
				"pkg/handlers/handlers.go": &fstest.MapFile{Data: []byte("package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")},
			}
			goModel, err := projectmodel.BuildGoModel(goSnapshot, arbitraryBoundaryParitySnapshotMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(goModel.Coverage.Complete).To(BeTrue())

			goChanges, goDiags := codesignal.EvaluateGoLayerViolations(goModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(goDiags).To(BeEmpty())
			Expect(goChanges).To(HaveLen(1))

			tsModel := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:3"},
				},
				Coverage: projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
			}
			tsChanges, tsDiags := codesignal.EvaluateTypeScriptLayerViolations(tsModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(tsDiags).To(BeEmpty())
			Expect(tsChanges).To(HaveLen(1))

			goReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-go"},
				ProjectChanges:  goChanges,
				ProjectCoverage: &goModel.Coverage,
			})
			tsReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-ts"},
				ProjectChanges:  tsChanges,
				ProjectCoverage: &tsModel.Coverage,
			})

			Expect(goReport.SchemaVersion).To(Equal("2"))
			Expect(tsReport.SchemaVersion).To(Equal("2"))
			Expect(goReport.ProjectChanges).To(HaveLen(1))
			Expect(tsReport.ProjectChanges).To(HaveLen(1))
			Expect(goReport.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(tsReport.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(goReport.ProjectSummary.ActiveChanges).To(Equal(1))
			Expect(tsReport.ProjectSummary.ActiveChanges).To(Equal(1))

			expectSharedArchitectureShape(goReport.ProjectChanges[0], tsReport.ProjectChanges[0], "architecture.layer_violation")
		})
	})

	When("a route handler has a statically resolved call path to a pinned sink, with no required-layer node on that path", func() {
		It("emits one baseline architecture.layer_bypass ProjectChange on both languages' Reports, identically shaped apart from language provenance", func() {
			ctx := context.Background()

			goSnapshot := fstest.MapFS{
				"go.mod":             &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
				"handler.go":         &fstest.MapFile{Data: []byte("package app\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\t\"example.com/app/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n\tdirectQuery()\n}\n\nfunc directQuery() {\n\trawQuery()\n}\n\nfunc rawQuery() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n")},
				"service/service.go": &fstest.MapFile{Data: []byte("package service\n\nimport \"database/sql\"\n\nfunc LoadUser() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n")},
			}
			goResult, err := projectmodel.BuildGoLayerBypass(ctx, goSnapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(goResult.Coverage.Complete).To(BeTrue())
			Expect(goResult.Witnesses).To(HaveLen(1), "expected exactly one deterministic bypass witness, got %+v", goResult.Witnesses)

			goChanges, goDiags := codesignal.EvaluateGoLayerBypass(goResult, "1", "backend-1", "digest-1")
			Expect(goDiags).To(BeEmpty())
			Expect(goChanges).To(HaveLen(1))

			tsResult := projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{{
					ID:            "bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSLayerBypassAlgorithm,
					Source:        "file:src/handlers/app.ts#getUsers",
					Sink:          "(PrismaClient).findMany",
					RequiredLayer: "service",
					Path: []projectmodel.LayerBypassStep{
						{NodeID: "file:src/handlers/app.ts#getUsers", Path: "src/handlers/app.ts"},
						{NodeID: "(PrismaClient).findMany"},
					},
					Confidence:       projectmodel.LayerBypassConfidenceHigh,
					AlgorithmVersion: projectmodel.TSLayerBypassAlgorithm,
				}},
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
			}
			tsChanges, tsDiags := codesignal.EvaluateTypeScriptLayerBypass(tsResult, "1", "backend-1", "digest-1")
			Expect(tsDiags).To(BeEmpty())
			Expect(tsChanges).To(HaveLen(1))

			goReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-go"},
				ProjectChanges:  goChanges,
				ProjectCoverage: &goResult.Coverage,
			})
			tsReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-ts"},
				ProjectChanges:  tsChanges,
				ProjectCoverage: &tsResult.Coverage,
			})

			Expect(goReport.ProjectChanges).To(HaveLen(1))
			Expect(tsReport.ProjectChanges).To(HaveLen(1))
			Expect(goReport.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(tsReport.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))

			expectSharedArchitectureShape(goReport.ProjectChanges[0], tsReport.ProjectChanges[0], "architecture.layer_bypass")

			Expect(goReport.ProjectChanges[0].PathSteps).NotTo(BeEmpty())
			Expect(tsReport.ProjectChanges[0].PathSteps).NotTo(BeEmpty())
			for _, step := range goReport.ProjectChanges[0].PathSteps {
				Expect(step.Confidence).To(Equal(codesignal.Confidence("high")))
			}
			for _, step := range tsReport.ProjectChanges[0].PathSteps {
				Expect(step.Confidence).To(Equal(codesignal.Confidence("high")))
			}
		})
	})

	When("a route handler has a statically resolved call path to a pinned sink, evidenced by real Go call-graph facts and an equivalent TypeScript sidecar-shaped fact", func() {
		It("surfaces the path as a ProjectFact only -- never a ProjectChange, Signal, or active-summary count -- on both languages' Reports (facts-only reachability, issue #216 AC-4/AC-11)", func() {
			ctx := context.Background()

			goSnapshot := fstest.MapFS{
				"go.mod": &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
				"handler.go": &fstest.MapFile{Data: []byte(
					"package app\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tloadUser()\n}\n\nfunc loadUser() {\n\tqueryDB()\n}\n\nfunc queryDB() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n",
				)},
			}
			goResult, err := projectmodel.BuildGoReachability(ctx, goSnapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(goResult.Coverage.Complete).To(BeTrue())
			Expect(goResult.Facts).To(HaveLen(1), "expected exactly one reachability fact, got %+v", goResult.Facts)

			goFacts := codesignal.ReachabilityProjectFacts(goResult, "go")

			tsResult := projectmodel.ReachabilityResult{
				Facts: []projectmodel.ReachabilityFact{{
					ID:         "reach:file:src/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSReachabilityAlgorithm,
					Kind:       projectmodel.KindPossibleCallReachability,
					Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
					Source:     "file:src/app.ts#getUsers",
					Sink:       "(PrismaClient).findMany",
					Path: []projectmodel.ReachabilityStep{
						{NodeID: "file:src/app.ts#getUsers"},
						{NodeID: "(PrismaClient).findMany"},
					},
					AlgorithmVersion: projectmodel.TSReachabilityAlgorithm,
				}},
				Algorithm: projectmodel.TSReachabilityAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
			}
			tsFacts := codesignal.ReachabilityProjectFacts(tsResult, "typescript")

			goReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-go"},
				ProjectFacts:    goFacts,
				ProjectCoverage: &goResult.Coverage,
			})
			tsReport := buildProjectReport(ctx, codesignal.Input{
				Scope:           codesignal.Scope{Revision: "rev-ts"},
				ProjectFacts:    tsFacts,
				ProjectCoverage: &tsResult.Coverage,
			})

			Expect(goReport.ProjectChanges).To(BeEmpty(), "a possible-call-reachability fact must never be reported as a compliance/bypass claim")
			Expect(tsReport.ProjectChanges).To(BeEmpty(), "a possible-call-reachability fact must never be reported as a compliance/bypass claim")
			Expect(goReport.Signals).To(BeEmpty())
			Expect(tsReport.Signals).To(BeEmpty())
			Expect(goReport.ProjectSummary.ActiveChanges).To(Equal(0))
			Expect(tsReport.ProjectSummary.ActiveChanges).To(Equal(0))

			Expect(goReport.ProjectFacts).To(HaveLen(1))
			Expect(tsReport.ProjectFacts).To(HaveLen(1))
			goFact, tsFact := goReport.ProjectFacts[0], tsReport.ProjectFacts[0]

			Expect(tsFact.Kind).To(Equal(goFact.Kind))
			Expect(tsFact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(tsFact.Provenance.Producer).To(Equal(goFact.Provenance.Producer))
			Expect(tsFact.Provenance.FindingKind).To(Equal(goFact.Provenance.FindingKind))
			Expect(tsFact.PathSteps[0].Confidence).To(Equal(goFact.PathSteps[0].Confidence))
			Expect(tsFact.PathSteps[0].Confidence).To(Equal(codesignal.Confidence("high")))

			Expect(goFact.Provenance.Language).To(Equal("go"))
			Expect(tsFact.Provenance.Language).To(Equal("typescript"))
		})
	})

	When("an import edge is type-only in TypeScript (Go has no equivalent syntax: the nearest parity-contract analog is a same-module import that never resolves to a real runtime package)", func() {
		It("never surfaces as architecture.layer_violation, while an otherwise-identical runtime/resolved edge still does (false-green control, in both languages)", func() {
			ctx := context.Background()

			tsForbiddenEdge := func(kind string) projectmodel.Model {
				return projectmodel.Model{
					ImportEdges: []projectmodel.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: kind, Resolution: "snapshot", Site: "pkg/handlers/h.ts:3"},
					},
					Coverage: projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
				}
			}

			typeOnlyModel := tsForbiddenEdge("type_only")
			typeOnlyChanges, typeOnlyDiags := codesignal.EvaluateTypeScriptLayerViolations(typeOnlyModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(typeOnlyDiags).To(BeEmpty())
			Expect(typeOnlyChanges).To(BeEmpty(), "a naive evaluator with no Kind filter would have surfaced this type-only edge")

			runtimeModel := tsForbiddenEdge("import")
			runtimeChanges, runtimeDiags := codesignal.EvaluateTypeScriptLayerViolations(runtimeModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(runtimeDiags).To(BeEmpty())
			Expect(runtimeChanges).To(HaveLen(1), "the otherwise-identical runtime import edge must still surface, proving the suppression above is real")

			typeOnlyReport := buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev-type-only"}, ProjectChanges: typeOnlyChanges, ProjectCoverage: &typeOnlyModel.Coverage})
			runtimeReport := buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev-runtime"}, ProjectChanges: runtimeChanges, ProjectCoverage: &runtimeModel.Coverage})

			Expect(typeOnlyReport.ProjectChanges).To(BeEmpty())
			Expect(typeOnlyReport.ProjectSummary.ActiveChanges).To(Equal(0))
			Expect(runtimeReport.ProjectChanges).To(HaveLen(1))
			Expect(runtimeReport.ProjectSummary.ActiveChanges).To(Equal(1))

			goUnresolvedSnapshot := fstest.MapFS{
				"go.mod":                   &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
				"pkg/db/db.go":             &fstest.MapFile{Data: []byte("package db\n\nvar Name = \"db\"\n")},
				"pkg/handlers/handlers.go": &fstest.MapFile{Data: []byte("package handlers\n\nimport \"example.com/app/pkg/db/missing\"\n\nfunc Use() string {\n\treturn missing.Name\n}\n")},
			}
			goUnresolvedModel, err := projectmodel.BuildGoModel(goUnresolvedSnapshot, arbitraryBoundaryParitySnapshotMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())
			goUnresolvedChanges, goUnresolvedDiags := codesignal.EvaluateGoLayerViolations(goUnresolvedModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(goUnresolvedDiags).To(BeEmpty())
			Expect(goUnresolvedChanges).To(BeEmpty(), "an import to a nonexistent same-module package must never resolve as an internal layer edge")

			goResolvedSnapshot := fstest.MapFS{
				"go.mod":                   &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
				"pkg/db/db.go":             &fstest.MapFile{Data: []byte("package db\n\nvar Name = \"db\"\n")},
				"pkg/handlers/handlers.go": &fstest.MapFile{Data: []byte("package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")},
			}
			goResolvedModel, err := projectmodel.BuildGoModel(goResolvedSnapshot, arbitraryBoundaryParitySnapshotMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())
			goResolvedChanges, goResolvedDiags := codesignal.EvaluateGoLayerViolations(goResolvedModel, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(goResolvedDiags).To(BeEmpty())
			Expect(goResolvedChanges).To(HaveLen(1), "the otherwise-identical resolved import must still surface, proving the suppression above is real")

			goUnresolvedReport := buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev-go-unresolved"}, ProjectChanges: goUnresolvedChanges, ProjectCoverage: &goUnresolvedModel.Coverage})
			goResolvedReport := buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev-go-resolved"}, ProjectChanges: goResolvedChanges, ProjectCoverage: &goResolvedModel.Coverage})

			Expect(goUnresolvedReport.ProjectChanges).To(BeEmpty())
			Expect(goUnresolvedReport.ProjectSummary.ActiveChanges).To(Equal(0))
			Expect(goResolvedReport.ProjectChanges).To(HaveLen(1))
			Expect(goResolvedReport.ProjectSummary.ActiveChanges).To(Equal(1))
		})
	})

	When("the same forbidden layer edge is reached through a Go import alias rather than an unaliased import", func() {
		It("resolves to a byte-identical architecture.layer_violation ProjectChange as the unaliased case", func() {
			ctx := context.Background()

			buildReportForHandlersSource := func(handlersSource string) *codesignal.Report {
				snapshot := fstest.MapFS{
					"go.mod":                   &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
					"pkg/db/db.go":             &fstest.MapFile{Data: []byte("package db\n\nvar Name = \"db\"\n")},
					"pkg/handlers/handlers.go": &fstest.MapFile{Data: []byte(handlersSource)},
				}
				model, err := projectmodel.BuildGoModel(snapshot, arbitraryBoundaryParitySnapshotMeta(), projectmodel.GoBuildOptions{})
				Expect(err).NotTo(HaveOccurred())
				changes, diags := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")
				Expect(diags).To(BeEmpty())
				Expect(changes).To(HaveLen(1))
				return buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev"}, ProjectChanges: changes, ProjectCoverage: &model.Coverage})
			}

			unaliasedReport := buildReportForHandlersSource("package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
			aliasedReport := buildReportForHandlersSource("package handlers\n\nimport dbalias \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn dbalias.Name\n}\n")

			Expect(aliasedReport.ProjectChanges).To(Equal(unaliasedReport.ProjectChanges), "a Go import alias must resolve to the identical package edge -- imports are classified by resolved path, never by the local alias identifier")
		})
	})

	// projectmodel.ImportEdge carries no field for the import statement's
	// original source specifier (e.g. a tsconfig path alias vs a relative
	// import) -- only the resolved target and the reporting site -- so this
	// spec cannot exercise specifier-transparency itself. What it does prove:
	// ProjectChange identity (RuleID/Kind/SemanticKey/Lifecycle/MachineEvidence)
	// depends only on the resolved edge, not on which site reported it --
	// the property specifier-transparency would actually rely on once the
	// sidecar resolves any specifier form to the same target.
	When("the same forbidden layer edge is reported at two different TypeScript import sites, already resolved to the identical file target", func() {
		It("resolves to the same shared rule id, semantic key, and lifecycle regardless of which site reported it", func() {
			ctx := context.Background()

			buildChangeForSite := func(site, to string) codesignal.ProjectChange {
				model := projectmodel.Model{
					ImportEdges: []projectmodel.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: to, Kind: "import", Resolution: "snapshot", Site: site},
					},
					Coverage: projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
				}
				changes, diags := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")
				Expect(diags).To(BeEmpty())
				Expect(changes).To(HaveLen(1))
				report := buildProjectReport(ctx, codesignal.Input{Scope: codesignal.Scope{Revision: "rev"}, ProjectChanges: changes, ProjectCoverage: &model.Coverage})
				Expect(report.ProjectChanges).To(HaveLen(1))
				return report.ProjectChanges[0]
			}

			firstSiteChange := buildChangeForSite("pkg/handlers/h.ts:3", "file:pkg/db/d.ts")
			secondSiteChange := buildChangeForSite("pkg/handlers/h.ts:5", "file:pkg/db/d.ts")

			Expect(firstSiteChange.RuleID).To(Equal(secondSiteChange.RuleID))
			Expect(firstSiteChange.Kind).To(Equal(secondSiteChange.Kind))
			Expect(firstSiteChange.SemanticKey).To(Equal(secondSiteChange.SemanticKey))
			Expect(firstSiteChange.Lifecycle).To(Equal(secondSiteChange.Lifecycle))
			Expect(firstSiteChange.MachineEvidence).To(Equal(secondSiteChange.MachineEvidence))
			Expect(firstSiteChange.PrimaryAnchor.Location.StartRow).NotTo(Equal(secondSiteChange.PrimaryAnchor.Location.StartRow), "sanity: the two fixtures must actually differ in reported site, or this test cannot distinguish real site-independence from two accidentally-identical inputs")
		})
	})
})
