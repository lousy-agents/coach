package githubingest

import (
	"context"
	"fmt"

	"github.com/google/go-github/v91/github"
)

// GitHubTreeRef identifies a repository at a ref whose file tree should be
// listed, as opposed to GitHubFileRef's single path.
type GitHubTreeRef struct{ Owner, Repo, Ref string }

// TreeEntry describes one matching file discovered by
// GitHubFileReader.ListFiles.
type TreeEntry struct {
	Path string `json:"path"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

// FileFilter reports whether path should be included in a ListFiles result.
// pkg/githubingest never imports pkg/semantics, so callers that want to
// restrict a listing to specific source languages (e.g. ".go", ".ts",
// ".tsx") must supply their own filter rather than relying on any built-in
// language list.
type FileFilter func(path string) bool

// TreeListOptions is GitHubFileReader.ListFiles's tree-walk budget and filter.
type TreeListOptions struct {
	// Filter, if non-nil, restricts the result to paths for which it
	// returns true. A nil Filter matches every regular file.
	Filter FileFilter

	// MaxFiles caps the number of matching files collected before the walk
	// aborts with an error wrapping ErrTooLarge. Zero means unlimited.
	MaxFiles int

	// MaxTotalBytes caps the sum of matching files' reported sizes before
	// the walk aborts with an error wrapping ErrTooLarge. Zero means
	// unlimited. Sizes come from the Contents API's directory-listing
	// "size" field, so this budget is checked without fetching any file's
	// content.
	MaxTotalBytes int64
}

// ListFiles recursively walks ref's repository tree via the Contents API,
// returning every regular file matching opts.Filter. An empty repository or
// an empty matching set returns an empty, non-nil slice and a nil error.
//
// The walk skips symlink and submodule entries entirely rather than
// following them, mirroring rejectIfPathIsSymlink's concern in reader.go: a
// symlink or submodule could otherwise be used to escape the repository
// boundary or double-count content.
func (r *GitHubFileReader) ListFiles(ctx context.Context, ref GitHubTreeRef, opts TreeListOptions) ([]TreeEntry, error) {
	walk := treeWalk{reader: r, entries: make([]TreeEntry, 0)}
	if err := walk.walk(ctx, ref, "", opts); err != nil {
		return nil, err
	}
	return walk.entries, nil
}

type treeWalk struct {
	reader     *GitHubFileReader
	entries    []TreeEntry
	totalBytes int64
}

func (w *treeWalk) walk(ctx context.Context, ref GitHubTreeRef, dir string, opts TreeListOptions) error {
	_, dirEntries, resp, err := w.reader.client.Repositories.GetContents(ctx, ref.Owner, ref.Repo, dir, &github.RepositoryContentGetOptions{Ref: ref.Ref})
	if err != nil {
		return mapContentsAPIError(err, resp, fmt.Sprintf("listing %s at ref %s", dirLabel(ref, dir), ref.Ref))
	}

	for _, entry := range dirEntries {
		if err := w.visitEntry(ctx, ref, opts, entry); err != nil {
			return err
		}
	}
	return nil
}

func (w *treeWalk) visitEntry(ctx context.Context, ref GitHubTreeRef, opts TreeListOptions, entry *github.RepositoryContent) error {
	switch entry.GetType() {
	case "dir":
		return w.walk(ctx, ref, entry.GetPath(), opts)
	case "file":
		return w.appendFile(ref, opts, entry)
	default:
		// symlink, submodule: skip. Do not recurse into or follow
		// these, to avoid escaping the repository boundary the same
		// way rejectIfPathIsSymlink's ReadFile check guards against.
		return nil
	}
}

func (w *treeWalk) appendFile(ref GitHubTreeRef, opts TreeListOptions, entry *github.RepositoryContent) error {
	if opts.Filter != nil && !opts.Filter(entry.GetPath()) {
		return nil
	}
	if opts.MaxFiles > 0 && len(w.entries)+1 > opts.MaxFiles {
		return fmt.Errorf("githubingest: tree listing for %s/%s at ref %s exceeds the configured file-count budget of %d: %w", ref.Owner, ref.Repo, ref.Ref, opts.MaxFiles, ErrTooLarge)
	}
	newTotal := w.totalBytes + int64(entry.GetSize())
	if opts.MaxTotalBytes > 0 && newTotal > opts.MaxTotalBytes {
		return fmt.Errorf("githubingest: tree listing for %s/%s at ref %s exceeds the configured byte budget of %d bytes: %w", ref.Owner, ref.Repo, ref.Ref, opts.MaxTotalBytes, ErrTooLarge)
	}
	w.totalBytes = newTotal
	w.entries = append(w.entries, TreeEntry{Path: entry.GetPath(), SHA: entry.GetSHA(), Size: entry.GetSize()})
	return nil
}

func dirLabel(ref GitHubTreeRef, dir string) string {
	if dir == "" {
		return fmt.Sprintf("%s/%s root", ref.Owner, ref.Repo)
	}
	return dir
}
