package codesignal_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

	scanner := &guardedSymbolScanner{hits: map[string][]string{}, patterns: patterns, symbols: symbols}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, scanner.visit)
		Expect(err).NotTo(HaveOccurred())
	}
	return scanner.hits
}

// guardedSymbolScanner is the filepath.WalkDirFunc state for
// scanForGuardedSymbols: patterns[i] matches symbols[i], and hits collects
// the symbols each visited .go file references.
type guardedSymbolScanner struct {
	hits     map[string][]string
	patterns []*regexp.Regexp
	symbols  []string
}

func (s *guardedSymbolScanner) visit(path string, entry os.DirEntry, err error) error {
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
	for i, symbol := range s.symbols {
		if s.patterns[i].Match(content) {
			s.hits[path] = append(s.hits[path], symbol)
		}
	}
	return nil
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
