package coachapi_test

import (
	"context"

	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func (w *memoryFencedWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	cap := &captureWriter{lease: w.lease}
	if err := cap.InsertDiagnostics(ctx, diagnostics); err != nil {
		return err
	}
	stamped := append([]coachapi.JobDiagnostic(nil), diagnostics...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

func handlerSourcedNames(calls []agentloop.RecordedCall) []string {
	var names []string
	for _, c := range calls {
		if c.Source == agentloop.CallSourceHandler {
			names = append(names, c.Name)
		}
	}
	return names
}

func findingUniqKey(f coachapi.JobFinding) string {
	rubric := ""
	if f.RubricID != nil {
		rubric = *f.RubricID
	}

	return string(f.Source) + "\x00" + rubric + "\x00" + f.PayloadHash
}

func (f *fakeTreeSource) ListFiles(_ context.Context, _, _, ref string, _ coachapi.BaselineListOptions) ([]coachapi.BaselineFileEntry, error) {
	f.listCalls++
	f.lastListRef = ref
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]coachapi.BaselineFileEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

// newGitHubBaselineTreeFixture builds a fakegithub Contents + commit graph that
// GitHubBaselineTreeSource can walk end-to-end (resolve → list → read).
// files keys must be top-level basenames (no nested paths).
func newGitHubBaselineTreeFixture(objectSHA string, files map[string][]byte) *fakegithub.Fixture {
	GinkgoHelper()
	fx := fakegithub.NewFixture("handler-github-tree-fixture")
	fx.Installation.Installations[42] = fakegithub.InstallationEntry{
		Token: "handler-install-token", Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
		InstallationID: 42, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Repos["acme/widgets"] = fakegithub.RepoMetaEntry{
		DefaultBranch: "main", Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/main"] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/"+objectSHA] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}

	rootKey := "acme/widgets/" + objectSHA
	rootEntries := make([]fakegithub.DirEntry, 0, len(files))
	i := 0
	for path, body := range files {
		Expect(path).NotTo(ContainSubstring("/"), "fixture helper supports top-level paths only")
		i++
		blob := fmt.Sprintf("blob%d", i)
		fx.Contents.Files[rootKey+"/"+path] = fakegithub.FileEntry{
			Content: body, SHA: blob, Scenario: fakegithub.ScenarioOK,
		}
		rootEntries = append(rootEntries, fakegithub.DirEntry{
			Name: path, Type: "file", SHA: blob, Size: len(body),
		})
	}

	fx.Contents.Dirs[rootKey] = rootEntries
	return &fx
}
