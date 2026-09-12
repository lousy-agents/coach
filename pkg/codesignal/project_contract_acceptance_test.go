package codesignal_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("project observation identity and lifecycle", func() {
	It("changes identity when rule, backend, or configuration identity changes", func() {
		base := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
		base.RuleVersion = "1"
		base.BackendVersion = "go-backend-v1"
		base.AlgorithmVersion = "cycle-v1"
		base.ConfigDigest = "config-a"

		variants := []codesignal.ProjectChange{base}
		for _, mutate := range []func(*codesignal.ProjectChange){
			func(change *codesignal.ProjectChange) { change.RuleVersion = "2" },
			func(change *codesignal.ProjectChange) { change.BackendVersion = "go-backend-v2" },
			func(change *codesignal.ProjectChange) { change.AlgorithmVersion = "cycle-v2" },
			func(change *codesignal.ProjectChange) { change.ConfigDigest = "config-b" },
		} {
			variant := base
			mutate(&variant)
			variants = append(variants, variant)
		}

		identities := make([]string, 0, len(variants))
		for _, change := range variants {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{change},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			identities = append(identities, report.ProjectChanges[0].ID+"/"+report.ProjectChanges[0].Fingerprint)
		}
		Expect(identities[1:]).NotTo(ContainElement(identities[0]))
	})

	It("derives changed from causal evidence rather than trusting a caller flag", func() {
		base := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
		base.CausalEvidenceDigest = "evidence-a"
		base.Changed = true
		head := base
		head.Changed = false

		report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:      []codesignal.ProjectChange{head},
			BaseProjectChanges:  []codesignal.ProjectChange{base},
			ProjectBaseAnalyzed: true,
			ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		})
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Changed).To(BeFalse())

		head.CausalEvidenceDigest = "evidence-b"
		report = build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:      []codesignal.ProjectChange{head},
			BaseProjectChanges:  []codesignal.ProjectChange{base},
			ProjectBaseAnalyzed: true,
			ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		})
		Expect(report.ProjectChanges[0].Changed).To(BeTrue())
	})

	It("keeps base-only observations unknown when lifecycle coverage is incomplete", func() {
		report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
			ProjectBaseAnalyzed: true,
			ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
		})

		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0))
		Expect(report.Diagnostics).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Kind": Equal("project_lifecycle_indeterminate"),
		})))
	})

	It("still treats non-empty base changes without ProjectBaseAnalyzed as lifecycle-indeterminate", func() {
		report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
			BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
			ProjectBaseAnalyzed: false,
			ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
		})
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		Expect(report.Diagnostics).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Kind":    Equal("project_lifecycle_indeterminate"),
			"Message": ContainSubstring("base coverage unavailable"),
		})))
	})

	// Nested related_locations / coverage_refs / path-step source_locations
	// are not path-ordered witness data: Build must canonicalize them so two
	// producers that emit the same observation in different traversal orders
	// still yield byte-identical schema-2 JSON (epic #208 array stability).
	It("canonicalizes nested project observation arrays independent of producer order", func() {
		left := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
		left.RelatedLocations = []codesignal.ProjectLocation{
			{Path: "z.go", Location: semantics.Location{StartRow: 2}},
			{Path: "a.go", Location: semantics.Location{StartRow: 1}},
		}
		left.CoverageRefs = []string{"z-phase", "a-phase"}
		left.PathSteps = []codesignal.ProjectPathStep{{
			NodeID: "package:pkg/a",
			SourceLocations: []codesignal.ProjectLocation{
				{Path: "z.go", Location: semantics.Location{StartRow: 9}},
				{Path: "a.go", Location: semantics.Location{StartRow: 1}},
			},
		}}

		right := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
		right.RelatedLocations = []codesignal.ProjectLocation{
			{Path: "a.go", Location: semantics.Location{StartRow: 1}},
			{Path: "z.go", Location: semantics.Location{StartRow: 2}},
		}
		right.CoverageRefs = []string{"a-phase", "z-phase"}
		right.PathSteps = []codesignal.ProjectPathStep{{
			NodeID: "package:pkg/a",
			SourceLocations: []codesignal.ProjectLocation{
				{Path: "a.go", Location: semantics.Location{StartRow: 1}},
				{Path: "z.go", Location: semantics.Location{StartRow: 9}},
			},
		}}

		cov := &projectmodel.Coverage{Phase: "full", Complete: true}
		leftReport := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:  []codesignal.ProjectChange{left},
			ProjectCoverage: cov,
		})
		rightReport := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:  []codesignal.ProjectChange{right},
			ProjectCoverage: cov,
		})

		leftJSON, err := json.Marshal(leftReport)
		Expect(err).NotTo(HaveOccurred())
		rightJSON, err := json.Marshal(rightReport)
		Expect(err).NotTo(HaveOccurred())
		Expect(leftJSON).To(Equal(rightJSON), "nested project observation arrays must be producer-order independent")
	})

	// classify/sort shallow-copy ProjectChange structs; PathSteps headers still
	// alias the caller's underlying array. Canonicalization must not reorder
	// the caller's SourceLocations in place.
	It("does not mutate the caller's project observation slices during Build", func() {
		change := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
		change.RelatedLocations = []codesignal.ProjectLocation{
			{Path: "z.go", Location: semantics.Location{StartRow: 2}},
			{Path: "a.go", Location: semantics.Location{StartRow: 1}},
		}
		change.PathSteps = []codesignal.ProjectPathStep{{
			NodeID: "package:pkg/a",
			SourceLocations: []codesignal.ProjectLocation{
				{Path: "z.go", Location: semantics.Location{StartRow: 9}},
				{Path: "a.go", Location: semantics.Location{StartRow: 1}},
			},
		}}
		fact := codesignal.ProjectFact{
			Kind:        "possible_call_reachability",
			SemanticKey: "reach:a->b",
			PathSteps: []codesignal.ProjectPathStep{{
				NodeID: "func:A",
				SourceLocations: []codesignal.ProjectLocation{
					{Path: "z.go", Location: semantics.Location{StartRow: 2}},
					{Path: "a.go", Location: semantics.Location{StartRow: 1}},
				},
			}},
			Provenance: codesignal.Provenance{Producer: "projectmodel"},
		}
		input := codesignal.Input{
			ProjectChanges:  []codesignal.ProjectChange{change},
			ProjectFacts:    []codesignal.ProjectFact{fact},
			ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		}
		callerRelated := input.ProjectChanges[0].RelatedLocations
		callerChangeSteps := input.ProjectChanges[0].PathSteps
		callerFactSteps := input.ProjectFacts[0].PathSteps

		_ = build(codesignal.Options{ProjectEnabled: true}, input)

		Expect(callerRelated[0].Path).To(Equal("z.go"), "caller RelatedLocations must keep producer order")
		Expect(callerChangeSteps[0].SourceLocations[0].Path).To(Equal("z.go"), "caller PathSteps SourceLocations must keep producer order")
		Expect(callerFactSteps[0].SourceLocations[0].Path).To(Equal("z.go"), "caller fact PathSteps SourceLocations must keep producer order")
	})

	It("canonicalizes nested project fact arrays independent of producer order", func() {
		left := codesignal.ProjectFact{
			Kind:         "possible_call_reachability",
			SemanticKey:  "reach:a->b",
			CoverageRefs: []string{"z", "a"},
			PathSteps: []codesignal.ProjectPathStep{{
				NodeID: "func:A",
				SourceLocations: []codesignal.ProjectLocation{
					{Path: "z.go", Location: semantics.Location{StartRow: 2}},
					{Path: "a.go", Location: semantics.Location{StartRow: 1}},
				},
			}},
			Provenance: codesignal.Provenance{Producer: "projectmodel"},
		}
		right := codesignal.ProjectFact{
			Kind:         "possible_call_reachability",
			SemanticKey:  "reach:a->b",
			CoverageRefs: []string{"a", "z"},
			PathSteps: []codesignal.ProjectPathStep{{
				NodeID: "func:A",
				SourceLocations: []codesignal.ProjectLocation{
					{Path: "a.go", Location: semantics.Location{StartRow: 1}},
					{Path: "z.go", Location: semantics.Location{StartRow: 2}},
				},
			}},
			Provenance: codesignal.Provenance{Producer: "projectmodel"},
		}

		cov := &projectmodel.Coverage{Phase: "full", Complete: true}
		leftReport := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectFacts:    []codesignal.ProjectFact{left},
			ProjectCoverage: cov,
		})
		rightReport := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectFacts:    []codesignal.ProjectFact{right},
			ProjectCoverage: cov,
		})

		leftJSON, err := json.Marshal(leftReport)
		Expect(err).NotTo(HaveOccurred())
		rightJSON, err := json.Marshal(rightReport)
		Expect(err).NotTo(HaveOccurred())
		Expect(leftJSON).To(Equal(rightJSON))
	})
})

// guardedUnwiredProjectSymbols lists every Go and TypeScript
// reachability/layer-bypass entry point that is deliberately library-only:
// none of these are wired into `coach codesignal` today (see README.md's
// "library-only" paragraph). BuildGoLayerBypass/EvaluateGoLayerBypass were
// wired into the real Go backend (issue #253) and are no longer guarded
// here; BuildGoReachability stays unwired precedent.
// EvaluateTypeScriptLayerBypass/ReachabilityProjectFacts (issue #216) were
// wired into the TypeScript backend via their own …FromModel-derived inputs
// (issue #331 T7, the TS equivalent of #253's Go wiring) and are no longer
// guarded here. BuildTypeScriptReachability and BuildTypeScriptLayerBypass
// themselves stay guarded: T7 wires only the FromModel variants
// (BuildTypeScriptReachabilityFromModel/BuildTypeScriptLayerBypassFromModel),
// which derive from an already-built Model; the bare names each still
// perform their own independent BuildTypeScriptModelViaSidecar round trip,
// so a CLI-layer reference to either would be a direct AC-RUN-5/AC-1
// regression.
var guardedUnwiredProjectSymbols = []string{
	"BuildGoReachability",
	"BuildTypeScriptReachability",
	"BuildTypeScriptLayerBypass",
}

func platformSurfaceGuardDirs() []string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return []string{
		filepath.Join(root, "internal", "codesignalcli"),
		filepath.Join(root, "cmd", "coach"),
	}
}

// scanForGuardedSymbols does a plain regex scan (no AST) of every .go file
// under each of dirs, recursively, for each of symbols, returning a map of
// file path -> symbols found. Each symbol is matched on word boundaries
// (\b), not as a bare substring, so a guarded bare name (e.g.
// "BuildTypeScriptReachability") never matches a longer identifier that
// merely starts with it (e.g. the wired
// "BuildTypeScriptReachabilityFromModel") -- see
// guardedUnwiredProjectSymbols's own doc comment for why that distinction
// matters here.
func scanForGuardedSymbols(dirs []string, symbols []string) map[string][]string {
	patterns := make([]*regexp.Regexp, len(symbols))
	for i, symbol := range symbols {
		patterns[i] = regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	}

	hits := map[string][]string{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, symbol := range symbols {
				if patterns[i].Match(content) {
					hits[path] = append(hits[path], symbol)
				}
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	}
	return hits
}

var _ = Describe("reachability/layer-bypass entry points remain library-only (issue #216 AC-9)", func() {
	It("has no reference to any unwired reachability/layer-bypass entry point from internal/codesignalcli or cmd/coach", func() {
		hits := scanForGuardedSymbols(platformSurfaceGuardDirs(), guardedUnwiredProjectSymbols)
		Expect(hits).To(BeEmpty(), "expected no CLI-layer references to library-only entry points, found: %+v", hits)
	})

	// "Does the guard actually guard something" proof (mirroring AC-7's
	// false-green-control spirit, applied to this static guard): a directory
	// that DOES reference a guarded symbol must fail the same assertion the
	// spec above makes. Exercised against a throwaway temp-dir fixture so
	// this permanent regression proof never depends on mutating and reverting
	// a real source file.
	It("would fail the same check if a CLI-layer file referenced a guarded symbol", func() {
		dir := GinkgoT().TempDir()
		fixture := "package codesignalcli\n\nimport \"github.com/lousy-agents/coach/pkg/projectmodel\"\n\nvar _ = projectmodel.BuildGoReachability\n"
		Expect(os.WriteFile(filepath.Join(dir, "fake_wire.go"), []byte(fixture), 0o644)).To(Succeed())

		hits := scanForGuardedSymbols([]string{dir}, guardedUnwiredProjectSymbols)
		Expect(hits).NotTo(BeEmpty(), "the guard must detect a reference when one exists")
	})

	// internal/codesignalcli and cmd/coach are not guaranteed to stay flat,
	// so the guard's recursion into subdirectories must also be proven.
	It("would fail the same check if a guarded symbol were referenced from a subdirectory", func() {
		dir := GinkgoT().TempDir()
		subdir := filepath.Join(dir, "wire")
		Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())
		fixture := "package wire\n\nimport \"github.com/lousy-agents/coach/pkg/projectmodel\"\n\nvar _ = projectmodel.BuildGoReachability\n"
		Expect(os.WriteFile(filepath.Join(subdir, "wire.go"), []byte(fixture), 0o644)).To(Succeed())

		hits := scanForGuardedSymbols([]string{dir}, guardedUnwiredProjectSymbols)
		Expect(hits).NotTo(BeEmpty(), "the guard must detect a reference nested under a subdirectory")
	})

	It("does not flag a CLI-layer file that only references the wired …FromModel variant", func() {
		dir := GinkgoT().TempDir()
		fixture := "package codesignalcli\n\nimport \"github.com/lousy-agents/coach/pkg/projectmodel\"\n\nvar _ = projectmodel.BuildTypeScriptReachabilityFromModel\nvar _ = projectmodel.BuildTypeScriptLayerBypassFromModel\n"
		Expect(os.WriteFile(filepath.Join(dir, "fake_wire.go"), []byte(fixture), 0o644)).To(Succeed())

		hits := scanForGuardedSymbols([]string{dir}, guardedUnwiredProjectSymbols)
		Expect(hits).To(BeEmpty(), "the …FromModel variants must not trip their bare-name siblings' guard, got %+v", hits)
	})
})
