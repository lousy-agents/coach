package codesignalcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var fakeTSSidecarBackendPath string

var _ = BeforeSuite(func() {
	dir, err := os.MkdirTemp("", "fake-ts-sidecar-backend-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	fakeTSSidecarBackendPath = filepath.Join(dir, "fake-ts-sidecar")
	root := filepath.Join("..", "..", "pkg", "projectmodel")
	build := exec.Command("go", "build", "-o", fakeTSSidecarBackendPath, "./testdata/fake_ts_sidecar")
	build.Dir = root
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building fake ts sidecar: %s", output)
})

var _ = Describe("tsProjectBackend.evaluateRevision", func() {
	When("the analyzer's response has root_scopes entries but is missing one of the policy's requested roots", func() {
		It("returns a real Go error wrapping ProjectScopeFromModel's mismatch, rather than soft-skipping to a nil project scope", func() {
			dir := acceptanceTempGitRepo()
			revision := acceptanceCommitFile(dir, "pkg/handlers/h.ts", "export const h = 1;\n")

			runtimeDir := GinkgoT().TempDir()
			runtime := &tsRuntime{
				ExecPath:    fakeTSSidecarBackendPath,
				ExecArgs:    []string{"--mode=root_scope_missing"},
				AnalyzerDir: runtimeDir,
			}

			roots := []string{".", "pkg/handlers"}
			policy := codesignal.LayerPolicy{}
			b := &tsProjectBackend{}

			_, _, _, _, scope, _, err := b.evaluateRevision(context.Background(), dir, revision, runtime, roots, policy, projectmodel.BypassLayer{}, false, "pcfg_test")

			Expect(err).To(HaveOccurred(), "expected a policy root with no matching root_scopes entry to surface as a real Go error, not a soft-skipped nil project scope")
			Expect(scope).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("deriving TypeScript project scope"))
			Expect(err.Error()).To(ContainSubstring("pkg/handlers"), "expected the error to name the unmatched policy root")

			var unresolved *CompilerUnresolvedError
			Expect(errors.As(err, &unresolved)).To(BeFalse(),
				"runBaselineAnalysis/runDiffAnalysis (cmd/coach/main.go) re-return an error to run() only when it is *CompilerUnresolvedError, which classifyAnalysisError then maps to exit 2; "+
					"every other error -- this one included -- is printed to stderr and swallowed to (nil, 0, nil), which run() turns into exit 1 via its report==nil fallback")
		})
	})
})
