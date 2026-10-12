package baseline_test

import (
	"context"

	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// fakeTreeSource is a test double for GitHub-backed tree fetch failures and budgets.
type fakeTreeSource struct {
	listErr        error
	readErr        error
	resolveErr     error
	resolvedSHA    string
	entries        []baseline.FileEntry
	contents       map[string][]byte
	listCalls      int
	readCalls      int
	resolveCalls   int
	lastListRef    string
	lastReadRef    string
	lastResolveRef string
}

func (f *fakeTreeSource) ResolveCommitSHA(_ context.Context, _, _, ref string) (string, error) {
	f.resolveCalls++
	f.lastResolveRef = ref
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	if f.resolvedSHA != "" {
		return f.resolvedSHA, nil
	}
	if ref == "" {
		return "HEAD", nil
	}
	return ref, nil
}

func (f *fakeTreeSource) ListFiles(_ context.Context, _, _, ref string, _ baseline.ListOptions) ([]baseline.FileEntry, error) {
	f.listCalls++
	f.lastListRef = ref
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]baseline.FileEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

func (f *fakeTreeSource) ReadFile(_ context.Context, _, _, ref, path string) ([]byte, string, error) {
	f.readCalls++
	f.lastReadRef = ref
	if f.readErr != nil {
		return nil, "", f.readErr
	}
	if f.contents == nil {
		return nil, "", githubingest.ErrNotFound
	}
	b, ok := f.contents[path]
	if !ok {
		return nil, "", githubingest.ErrNotFound
	}
	return append([]byte(nil), b...), "blob-sha", nil
}
