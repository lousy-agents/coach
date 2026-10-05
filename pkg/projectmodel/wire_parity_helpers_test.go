package projectmodel

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// goWireFields returns structType's exported fields' wireField values in
// declaration order.
func goWireFields(t *testing.T, structType reflect.Type) []wireField {
	t.Helper()
	fields := make([]wireField, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" {
			t.Fatalf("%s field %q has no json tag", structType, f.Name)
		}
		name, opts, _ := strings.Cut(tag, ",")
		fields = append(fields, wireField{Name: name, Optional: strings.Contains(opts, "omitempty")})
	}
	return fields
}

// tsInterfaceFields returns interfaceName's field list recovered from an
// "export interface Name { ... }" block in source. The body is captured
// with a negated class ([^}]*), so it stops at the first "}"; this assumes
// (true for protocol.ts today) no interface body contains a nested "}".
func tsInterfaceFields(t *testing.T, source []byte, interfaceName string) []wireField {
	t.Helper()
	blockPattern := regexp.MustCompile(`export interface ` + regexp.QuoteMeta(interfaceName) + `\s*\{([^}]*)\}`)
	m := blockPattern.FindSubmatch(source)
	if m == nil {
		t.Fatalf("protocol.ts: no %q interface found; the wire-protocol call-graph/reachability/bypass fields (issue #216 Task 1) are not implemented yet", interfaceName)
	}
	matches := tsInterfaceFieldPattern.FindAllSubmatch(m[1], -1)
	fields := make([]wireField, 0, len(matches))
	for _, fm := range matches {
		fields = append(fields, wireField{Name: string(fm[1]), Optional: string(fm[2]) == "?"})
	}
	return fields
}

// readProtocolTSSource reads js/semantics/src/project-sidecar/protocol.ts
// relative to this test file's own path, mirroring
// ts_sidecar_integration_fixtures_test.go's repoRootFromThisFile
// convention (that helper lives in the projectmodel_test package, so it is
// not reachable from here).
func readProtocolTSSource(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	path := filepath.Join(repoRoot, "js", "semantics", "src", "project-sidecar", "protocol.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %s", path, err)
	}
	return data
}

func readReachabilityRegistryTSSource(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	path := filepath.Join(repoRoot, "js", "semantics", "src", "project-sidecar", "reachability-registry.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %s", path, err)
	}
	return data
}
