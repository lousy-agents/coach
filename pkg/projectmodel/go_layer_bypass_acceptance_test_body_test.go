package projectmodel_test

import (
	"context"
	"os"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_goLayerBypassAcceptanceTest_producesExactlyOneWitnessWhosePathReflectsTheDir_52(requiredServiceLayer projectmodel.BypassLayer) {
	snapshot := os.DirFS("testdata/go_layer_bypass_compliant_and_bypass")
	result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
		RequiredLayer: requiredServiceLayer,
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(result.Witnesses).To(HaveLen(1), "expected exactly one bypass witness, got %+v", result.Witnesses)
	witness := result.Witnesses[0]

	Expect(witness.ID).To(Equal("bypass:service:example.com/layerbypassmixed.Handler->(*database/sql.DB).Query@" + projectmodel.LayerBypassAlgorithm))
	Expect(witness.Source).To(Equal("example.com/layerbypassmixed.Handler"))
	Expect(witness.Sink).To(Equal("(*database/sql.DB).Query"))
	Expect(witness.RequiredLayer).To(Equal("service"))
	Expect(witness.Confidence).To(Equal(projectmodel.LayerBypassConfidenceHigh))
	Expect(witness.AlgorithmVersion).To(Equal(projectmodel.LayerBypassAlgorithm))

	// False-green control: the fixture's compliant route
	// (Handler -> service.LoadUser -> sink, 3 hops) is strictly
	// shorter than its bypass route (Handler -> directQuery ->
	// rawQuery -> sink, 4 hops), so a broken implementation that
	// leaves the required layer's nodes in the graph (a no-op
	// removal) would have BFS pick the shorter, still-present
	// compliant route instead -- failing this exact-path assertion
	// rather than happening to agree with it.
	Expect(witness.Path).To(HaveLen(4), "expected Handler, directQuery, rawQuery, and the sink, got %+v", witness.Path)
	Expect(witness.Path[0].NodeID).To(Equal("example.com/layerbypassmixed.Handler"))
	Expect(witness.Path[1].NodeID).To(Equal("example.com/layerbypassmixed.directQuery"))
	Expect(witness.Path[2].NodeID).To(Equal("example.com/layerbypassmixed.rawQuery"))
	Expect(witness.Path[3].NodeID).To(Equal("(*database/sql.DB).Query"))
	for _, step := range witness.Path {
		Expect(step.NodeID).NotTo(ContainSubstring("/service."), "witness path must never route through the required service layer, got %+v", witness.Path)
	}

	// Every local-function step resolves a real repository-relative
	// declaration position (see
	// testdata/go_layer_bypass_compliant_and_bypass/main.go), not just the
	// SSA node identity -- proving BuildGoLayerBypass surfaces real source
	// data rather than requiring a downstream consumer to fabricate one.
	Expect(witness.Path[0].Path).To(Equal("main.go"))
	Expect(witness.Path[0].Line).To(Equal(17), "expected Handler's func declaration line")
	Expect(witness.Path[1].Path).To(Equal("main.go"))
	Expect(witness.Path[1].Line).To(Equal(22), "expected directQuery's func declaration line")
	Expect(witness.Path[2].Path).To(Equal("main.go"))
	Expect(witness.Path[2].Line).To(Equal(26), "expected rawQuery's func declaration line")
	// The sink is a stdlib function with no declaration in the snapshot,
	// so it must never carry a fabricated position.
	Expect(witness.Path[3].Path).To(BeEmpty())
	Expect(witness.Path[3].Line).To(BeZero())

	Expect(result.Coverage.Complete).To(BeTrue())
}
