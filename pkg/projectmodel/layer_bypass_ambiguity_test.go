package projectmodel

import (
	"slices"
	"testing"
	"testing/fstest"
)

type ambiguityCase struct {
	name         string
	ambiguity    layerAmbiguity
	unclassified bool
	want         []string
}

func ambiguityRow(name string, ambiguity layerAmbiguity, unclassified bool, want ...string) ambiguityCase {
	return ambiguityCase{name: name, ambiguity: ambiguity, unclassified: unclassified, want: want}
}

func TestLayerAmbiguityDiagnostics(t *testing.T) {
	applied, unmatched, otherReason := layerAmbiguity{}, layerAmbiguity{ambiguous: true, unmatched: true}, layerAmbiguity{ambiguous: true}
	both := []string{DiagLayerBypassAmbiguousLayer, DiagLayerBypassRequiredLayerUnmatched}
	tests := []ambiguityCase{
		ambiguityRow("nothing is ambiguous", applied, false),
		ambiguityRow("a matched layer with a node outside every layer is ambiguous without the unmatched cause", applied, true, DiagLayerBypassAmbiguousLayer),
		ambiguityRow("a layer under which nothing sits names the cause", unmatched, false, both...),
		ambiguityRow("a layer that is ambiguous for another reason names no cause", otherReason, false, DiagLayerBypassAmbiguousLayer),
		ambiguityRow("an unclassified node does not repeat the ambiguous code", unmatched, true, both...),
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

var serviceLayer = BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}}

func (c ambiguityCase) run(t *testing.T) {
	expectDiagnosticCodes(t, c.ambiguity.diagnostics(nil, serviceLayer, c.unclassified), c.want)
}

func expectDiagnosticCodes(t *testing.T, diagnostics []Diagnostic, want []string) {
	t.Helper()
	if got := diagnosticCodes(diagnostics); !slices.Equal(got, want) {
		t.Fatalf("diagnostic codes = %v, want %v", got, want)
	}
}

type packageCase struct {
	name  string
	files []string
	want  bool
}

func packageRow(name string, want bool, files ...string) packageCase {
	return packageCase{name: name, files: files, want: want}
}

func TestGoLayerHasPackage(t *testing.T) {
	tests := []packageCase{
		packageRow("a source file in the layer's directory", true, "pkg/service/types.go"),
		packageRow("a source file in a directory below it", true, "pkg/service/inner/types.go"),
		packageRow("only a test file in the layer's directory", false, "pkg/service/types_test.go"),
		packageRow("a source file outside the layer", false, "pkg/other/types.go"),
		packageRow("a file that is not Go source", false, "pkg/service/README.md"),
		packageRow("a directory that only shares a stem with the prefix", false, "pkg/servicex/types.go"),
		packageRow("vendored code under the prefix", false, "pkg/service/vendor/dep/dep.go"),
		packageRow("testdata under the prefix", false, "pkg/service/testdata/case.go"),
		packageRow("a hidden directory under the prefix", false, "pkg/service/.cache/gen.go"),
		packageRow("a directory the go tool ignores because of its leading underscore", false, "pkg/service/_attic/old.go"),
		packageRow("a matching file followed by files that do not match", true, "pkg/service/a.go", "pkg/store/s.go", "zz/z.go"),
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

func (c packageCase) run(t *testing.T) {
	expectLayerHasPackage(t, c.files, serviceLayer, c.want)
}

func expectLayerHasPackage(t *testing.T, files []string, layer BypassLayer, want bool) {
	t.Helper()
	if got := goLayerHasPackage(snapshotOf(files), layer, nil); got != want {
		t.Fatalf("goLayerHasPackage(%v) = %v, want %v", files, got, want)
	}
}

func diagnosticCodes(diagnostics []Diagnostic) []string {
	var codes []string
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func snapshotOf(files []string) fstest.MapFS {
	snapshot := fstest.MapFS{}
	for _, name := range files {
		snapshot[name] = &fstest.MapFile{Data: []byte("package x\n")}
	}
	return snapshot
}

func TestGoLayerHasPackageWithinRoots(t *testing.T) {
	files := []string{"pkg/handlers/h.go", "pkg/service/svc.go"}
	snapshot := snapshotOf(files)
	for _, test := range []struct {
		roots []string
		want  bool
	}{
		{nil, true},
		{[]string{"."}, true},
		{[]string{"pkg/handlers", "pkg/service"}, true},
		{[]string{"pkg/handlers"}, false},
		{[]string{"pkg/missing"}, false},
		{[]string{"./pkg/service"}, true},
		{[]string{"pkg/service/"}, true},
	} {
		if got := goLayerHasPackage(snapshot, serviceLayer, test.roots); got != test.want {
			t.Errorf("goLayerHasPackage(roots=%v) = %v, want %v", test.roots, got, test.want)
		}
	}
}
