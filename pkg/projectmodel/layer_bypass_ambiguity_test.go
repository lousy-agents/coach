package projectmodel

import (
	"testing"
	"testing/fstest"
)

func TestLayerAmbiguityDiagnostics(t *testing.T) {
	layer := BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}}
	tests := []struct {
		name         string
		ambiguity    layerAmbiguity
		unclassified bool
		want         []string
	}{
		{"nothing is ambiguous", layerAmbiguity{}, false, nil},
		{"a matched layer with a node outside every layer is ambiguous without the unmatched cause", layerAmbiguity{}, true, []string{DiagLayerBypassAmbiguousLayer}},
		{"a layer under which nothing sits names the cause", layerAmbiguity{ambiguous: true, unmatched: true}, false, []string{DiagLayerBypassAmbiguousLayer, DiagLayerBypassRequiredLayerUnmatched}},
		{"a layer that is ambiguous for another reason names no cause", layerAmbiguity{ambiguous: true}, false, []string{DiagLayerBypassAmbiguousLayer}},
		{"an unclassified node does not repeat the ambiguous code", layerAmbiguity{ambiguous: true, unmatched: true}, true, []string{DiagLayerBypassAmbiguousLayer, DiagLayerBypassRequiredLayerUnmatched}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, diagnostic := range test.ambiguity.diagnostics(nil, layer, test.unclassified) {
				got = append(got, diagnostic.Code)
			}
			if len(got) != len(test.want) {
				t.Fatalf("diagnostic codes = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("diagnostic codes = %v, want %v", got, test.want)
				}
			}
		})
	}
}

func TestGoLayerHasPackage(t *testing.T) {
	layer := BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}}
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"a source file in the layer's directory", []string{"pkg/service/types.go"}, true},
		{"a source file in a directory below it", []string{"pkg/service/inner/types.go"}, true},
		{"only a test file in the layer's directory", []string{"pkg/service/types_test.go"}, false},
		{"a source file outside the layer", []string{"pkg/other/types.go"}, false},
		{"a file that is not Go source", []string{"pkg/service/README.md"}, false},
		{"a directory that only shares a stem with the prefix", []string{"pkg/servicex/types.go"}, false},
		{"vendored code under the prefix", []string{"pkg/service/vendor/dep/dep.go"}, false},
		{"testdata under the prefix", []string{"pkg/service/testdata/case.go"}, false},
		{"a hidden directory under the prefix", []string{"pkg/service/.cache/gen.go"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := fstest.MapFS{}
			for _, name := range test.files {
				snapshot[name] = &fstest.MapFile{Data: []byte("package x\n")}
			}
			if got := goLayerHasPackage(snapshot, layer); got != test.want {
				t.Fatalf("goLayerHasPackage(%v) = %v, want %v", test.files, got, test.want)
			}
		})
	}
}
