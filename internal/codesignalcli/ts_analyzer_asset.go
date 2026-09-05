package codesignalcli

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// tsAnalyzerAssetFiles embeds internal/codesignalcli/tsanalyzerasset/, a
// checked-in, generated copy of js/semantics' compiled project-sidecar
// build output (js/semantics/bin/coach-ts-project-sidecar plus
// bin/project-sidecar/), synced by
// js/semantics/scripts/sync-ts-analyzer-embed.mjs (mise's
// ts-analyzer-embed-sync task). Do not hand-edit files under
// tsanalyzerasset/: mise's ts-analyzer-embed-stale-check task (wired into
// js-ci) re-syncs from a fresh js/semantics build and fails CI on any
// drift, the same tidy-check pattern applied to go.mod/go.sum. go:embed
// cannot reach across the js/semantics/ package boundary with `..`, which
// is why this checked-in copy exists instead of embedding js/semantics/bin
// directly.
//
//go:embed all:tsanalyzerasset
var tsAnalyzerAssetFiles embed.FS

// tsAnalyzerShimAssetPath is the embedded asset's entry-point shim, the
// only file materialization marks executable. embed.FS never reports a
// source file's executable bit (every embedded regular file's FileInfo.Mode
// reads as 0444, directories as 0555), so the executable bit cannot be
// recovered by inspecting embedded FileInfo at materialization time -- it
// must be reasserted for this one known path instead.
const tsAnalyzerShimAssetPath = "coach-ts-project-sidecar"

var tsAnalyzerAssetFS fs.FS = mustSubFS(tsAnalyzerAssetFiles, "tsanalyzerasset")

func mustSubFS(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(fmt.Sprintf("coach: embedded TypeScript analyzer asset missing %s/ root: %v", dir, err))
	}
	return sub
}

// TSAnalyzerAssetDigest returns a deterministic sha256 hex digest over
// every file in the embedded TypeScript analyzer asset (path and content,
// visited in path-sorted order), so the digest is stable across process
// runs and only changes when the generated asset's content changes.
func TSAnalyzerAssetDigest() (string, error) {
	return digestFS(tsAnalyzerAssetFS)
}

func digestFS(src fs.FS) (string, error) {
	var paths []string
	if err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	}); err != nil {
		return "", err
	}

	h := sha256.New()
	for _, p := range paths {
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// materializeMkdirTemp creates the private directory each MaterializeTSAnalyzer
// call copies the embedded asset into. It is a package-level var (test seam,
// matching runSnapshotGit/snapshotGitCommandContext in project_snapshot.go)
// so acceptance specs can capture the exact directory materializeFS creates
// and assert it is removed after a failure or a cancelled context.
var materializeMkdirTemp = os.MkdirTemp

// MaterializeTSAnalyzer copies the embedded, generated TypeScript analyzer
// asset into a new private temporary directory and returns that directory
// alongside a cleanup func that removes it. On error (including ctx
// cancellation), dir is "" and no directory is created or left behind --
// the returned cleanup is a no-op and callers need not (and cannot
// usefully) call it. On success the caller owns dir and must call cleanup,
// which is safe to call more than once, once it is done with the
// directory.
func MaterializeTSAnalyzer(ctx context.Context) (dir string, cleanup func(), err error) {
	return materializeFS(ctx, tsAnalyzerAssetFS)
}

func materializeFS(ctx context.Context, src fs.FS) (string, func(), error) {
	dir, err := materializeMkdirTemp("", "coach-ts-analyzer-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("coach: creating TypeScript analyzer materialization directory: %w", err)
	}

	walkErr := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		target := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, readErr := fs.ReadFile(src, p)
		if readErr != nil {
			return readErr
		}
		mode := os.FileMode(0o644)
		if p == tsAnalyzerShimAssetPath {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
	if walkErr != nil {
		os.RemoveAll(dir)
		return "", func() {}, fmt.Errorf("coach: materializing TypeScript analyzer asset: %w", walkErr)
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}
