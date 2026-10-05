package projectmodel_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func expectLargeSnapshotYieldsOneCompleteModel(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	const fileCount = 80
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
	}
	for i := 0; i < fileCount; i++ {
		var body string
		if i == fileCount-1 {
			body = fmt.Sprintf("export const v%d = %d;\n", i, i)
		} else {
			body = fmt.Sprintf("export { v%d } from \"./m%d\";\nexport const v%d = %d;\n", i+1, i+1, i, i)
		}
		snapshot[fmt.Sprintf("src/m%d.ts", i)] = file(body)
	}

	start := time.Now()
	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	elapsed := time.Since(start)

	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)
	Expect(model.ImportEdges).To(HaveLen(fileCount-1), "expected one re-export edge per chained file")
	Expect(elapsed).To(BeNumerically("<", 20*time.Second), "analysis of %d trivial files took %s", fileCount, elapsed)
}

func expectBackendDigestCarriedThroughUnchanged(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {

	vendoredDir := filepath.Join(jsSemanticsRoot(), "bin", "project-sidecar")
	sourceDigest := sidecarSourceDigest(vendoredDir)

	typescriptVersion := readJSSemanticsTypescriptDevDependency()
	backendDigest := fmt.Sprintf("ts-sidecar:sha256:%s;protocol:%d;typescript:%s", sourceDigest, projectbridge.ProtocolVersion, typescriptVersion)

	meta := testMeta()
	meta.BackendDigest = backendDigest

	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"src/a.ts": file("export const a = 1;\n"),
	}

	first, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(first.Snapshot.BackendDigest).To(Equal(backendDigest))
	Expect(first.Snapshot.BackendDigest).To(ContainSubstring(fmt.Sprintf("protocol:%d", projectbridge.ProtocolVersion)))
	Expect(first.Snapshot.BackendDigest).To(ContainSubstring("typescript:" + typescriptVersion))

	second, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(second.Snapshot.BackendDigest).To(Equal(first.Snapshot.BackendDigest), "expected the sidecar/protocol/toolchain identity to be stable across calls")

	mutatedDir, mkErr := os.MkdirTemp("", "ts-sidecar-source-mutation-*")
	Expect(mkErr).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, mutatedDir)
	Expect(copyDirRecursive(vendoredDir, mutatedDir)).To(Succeed())

	var firstJSFile string
	Expect(filepath.WalkDir(mutatedDir, func(p string, d fs.DirEntry, walkErr error) error {
		Expect(walkErr).NotTo(HaveOccurred())
		if !d.IsDir() && strings.HasSuffix(p, ".js") && firstJSFile == "" {
			firstJSFile = p
		}
		return nil
	})).To(Succeed())
	Expect(firstJSFile).NotTo(BeEmpty(), "expected at least one vendored .js file to mutate")

	original, readErr := os.ReadFile(firstJSFile)
	Expect(readErr).NotTo(HaveOccurred())
	Expect(os.WriteFile(firstJSFile, append(original, []byte("\n// mutated for coverage-identity test\n")...), 0o644)).To(Succeed())

	mutatedDigest := sidecarSourceDigest(mutatedDir)
	Expect(mutatedDigest).NotTo(Equal(sourceDigest), "expected the digest to change when a vendored sidecar source file changes")
}
