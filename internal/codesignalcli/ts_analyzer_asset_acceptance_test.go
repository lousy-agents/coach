package codesignalcli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type delayedFS struct {
	fs.FS
	onOpen func(name string) error
}

func (d delayedFS) Open(name string) (fs.File, error) {
	if d.onOpen != nil {
		if err := d.onOpen(name); err != nil {
			return nil, err
		}
	}
	return d.FS.Open(name)
}

func fakeTSAnalyzerFS() fs.FS {
	return fstest.MapFS{
		"coach-ts-project-sidecar":       &fstest.MapFile{Data: []byte("#!/usr/bin/env node\n"), Mode: 0o755},
		"project-sidecar/main.js":        &fstest.MapFile{Data: []byte("export {};\n")},
		"project-sidecar/analyze.js":     &fstest.MapFile{Data: []byte("export function analyze() {}\n")},
		"project-sidecar/resolve.js":     &fstest.MapFile{Data: []byte("export function resolve() {}\n")},
		"project-sidecar/nested/deep.js": &fstest.MapFile{Data: []byte("export const x = 1;\n")},
	}
}

var _ = Describe("TypeScript analyzer asset", func() {
	Describe("TSAnalyzerAssetDigest", func() {
		It("is deterministic across repeated calls over the same embedded tree", func() {
			d1, err := TSAnalyzerAssetDigest()
			Expect(err).NotTo(HaveOccurred())
			d2, err := TSAnalyzerAssetDigest()
			Expect(err).NotTo(HaveOccurred())
			Expect(d1).To(Equal(d2))
			Expect(d1).NotTo(BeEmpty())
		})

		It("changes when the underlying file tree's content changes", func() {
			fsA := fstest.MapFS{"f.js": &fstest.MapFile{Data: []byte("a")}}
			fsB := fstest.MapFS{"f.js": &fstest.MapFile{Data: []byte("b")}}

			digestA, err := digestFS(fsA)
			Expect(err).NotTo(HaveOccurred())
			digestB, err := digestFS(fsB)
			Expect(err).NotTo(HaveOccurred())

			Expect(digestA).NotTo(Equal(digestB))
		})
	})

	Describe("MaterializeTSAnalyzer", func() {
		It("produces a private temp directory whose contents are byte-identical to the embedded generated asset", func() {
			dir, cleanup, err := MaterializeTSAnalyzer(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer cleanup()

			var got []string
			Expect(fs.WalkDir(tsAnalyzerAssetFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
				Expect(walkErr).NotTo(HaveOccurred())
				if d.IsDir() {
					return nil
				}
				got = append(got, p)

				want, readErr := fs.ReadFile(tsAnalyzerAssetFS, p)
				Expect(readErr).NotTo(HaveOccurred())

				have, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
				Expect(readErr).NotTo(HaveOccurred(), "materialized file missing: %s", p)
				Expect(have).To(Equal(want), "content mismatch for %s", p)
				return nil
			})).To(Succeed())
			Expect(got).NotTo(BeEmpty())

			shimInfo, err := os.Stat(filepath.Join(dir, "coach-ts-project-sidecar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(shimInfo.Mode()&0o111).NotTo(BeZero(), "materialized sidecar shim must be executable (embed.FS itself never reports the source's executable bit)")

			pkg, err := os.ReadFile(filepath.Join(dir, "package.json"))
			Expect(err).NotTo(HaveOccurred(), "materialized analyzer root must carry package.json so Node does not walk above it for a package scope")
			Expect(string(pkg)).To(Equal("{\"type\":\"module\"}\n"))
		})

		It("removes the materialized directory once the caller invokes cleanup after success", func() {
			dir, cleanup, err := MaterializeTSAnalyzer(context.Background())
			Expect(err).NotTo(HaveOccurred())

			_, statErr := os.Stat(dir)
			Expect(statErr).NotTo(HaveOccurred())

			cleanup()

			_, statErr = os.Stat(dir)
			Expect(errors.Is(statErr, fs.ErrNotExist)).To(BeTrue(), "directory must be gone after cleanup: %v", statErr)
		})

		It("removes any partial directory when materialization fails mid-copy", func() {
			var capturedDir string
			var mu sync.Mutex
			original := materializeMkdirTemp
			DeferCleanup(func() { materializeMkdirTemp = original })
			materializeMkdirTemp = func(dir, pattern string) (string, error) {
				d, err := original(dir, pattern)
				mu.Lock()
				capturedDir = d
				mu.Unlock()
				return d, err
			}

			boom := errors.New("simulated read failure")
			src := delayedFS{
				FS: fakeTSAnalyzerFS(),
				onOpen: func(name string) error {
					if name == "project-sidecar/resolve.js" {
						return boom
					}
					return nil
				},
			}

			_, _, err := materializeFS(context.Background(), src)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, boom)).To(BeTrue())

			mu.Lock()
			dir := capturedDir
			mu.Unlock()
			Expect(dir).NotTo(BeEmpty())
			_, statErr := os.Stat(dir)
			Expect(errors.Is(statErr, fs.ErrNotExist)).To(BeTrue(), "partial directory must not survive a mid-copy failure: %v", statErr)
		})

		It("removes the materialized directory when the context is cancelled mid-materialization", func() {
			var capturedDir string
			var mu sync.Mutex
			original := materializeMkdirTemp
			DeferCleanup(func() { materializeMkdirTemp = original })
			materializeMkdirTemp = func(dir, pattern string) (string, error) {
				d, err := original(dir, pattern)
				mu.Lock()
				capturedDir = d
				mu.Unlock()
				return d, err
			}

			ctx, cancel := context.WithCancel(context.Background())
			opened := make(chan struct{}, 8)
			src := delayedFS{
				FS: fakeTSAnalyzerFS(),
				onOpen: func(name string) error {
					opened <- struct{}{}
					time.Sleep(20 * time.Millisecond)
					return nil
				},
			}

			go func() {
				<-opened
				<-opened
				cancel()
			}()

			_, _, err := materializeFS(ctx, src)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, context.Canceled)).To(BeTrue(), "got: %v", err)

			mu.Lock()
			dir := capturedDir
			mu.Unlock()
			Expect(dir).NotTo(BeEmpty())
			_, statErr := os.Stat(dir)
			Expect(errors.Is(statErr, fs.ErrNotExist)).To(BeTrue(), "directory must not survive ctx cancellation mid-materialization: %v", statErr)
		})
	})
})
