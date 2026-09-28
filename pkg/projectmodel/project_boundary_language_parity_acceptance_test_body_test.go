package projectmodel_test

import (
	"context"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectBoundaryLanguageParityAcceptanceTest_emitsOneBaselineArchitectureLayerBypassProjectCh_140() {
	ctx := context.Background()

	goSnapshot := fstest.MapFS{
		"go.mod":             &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25\n")},
		"handler.go":         &fstest.MapFile{Data: []byte("package app\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\t\"example.com/app/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n\tdirectQuery()\n}\n\nfunc directQuery() {\n\trawQuery()\n}\n\nfunc rawQuery() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n")},
		"service/service.go": &fstest.MapFile{Data: []byte("package service\n\nimport \"database/sql\"\n\nfunc LoadUser() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n")},
	}
	goResult, err := projectmodel.BuildGoLayerBypass(ctx, goSnapshot, projectmodel.LayerBypassOptions{
		RequiredLayer: domain.BypassLayer{Name: "service", Prefixes: []string{"service"}},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(goResult.Coverage.Complete).To(BeTrue())
	Expect(goResult.Witnesses).To(HaveLen(1), "expected exactly one deterministic bypass witness, got %+v", goResult.Witnesses)

	goChanges, goDiags := codesignal.EvaluateGoLayerBypass(goResult, "1", "backend-1", "digest-1")
	Expect(goDiags).To(BeEmpty())
	Expect(goChanges).To(HaveLen(1))

	tsResult := domain.LayerBypassResult{
		Witnesses: []domain.LayerBypassWitness{{
			ID:            "bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + domain.TSLayerBypassAlgorithm,
			Source:        "file:src/handlers/app.ts#getUsers",
			Sink:          "(PrismaClient).findMany",
			RequiredLayer: "service",
			Path: []domain.LayerBypassStep{
				{NodeID: "file:src/handlers/app.ts#getUsers", Path: "src/handlers/app.ts"},
				{NodeID: "(PrismaClient).findMany"},
			},
			Confidence:       domain.LayerBypassConfidenceHigh,
			AlgorithmVersion: domain.TSLayerBypassAlgorithm,
		}},
		Algorithm: domain.TSLayerBypassAlgorithm,
		Coverage:  domain.Coverage{Phase: "ts_sidecar_build", Complete: true},
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
}
