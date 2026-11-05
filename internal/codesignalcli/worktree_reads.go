package codesignalcli

import (
	"io/fs"
	"sync"
)

// readRecorder remembers which names the analysis opened or read through the
// overlay, so the report can list the configuration files it actually read
// rather than every one that happens to differ on disk.
type readRecorder struct {
	fs.FS
	mu     sync.Mutex
	opened map[string]bool
}

func newReadRecorder(inner fs.FS) *readRecorder {
	return &readRecorder{FS: inner, opened: map[string]bool{}}
}

func (r *readRecorder) note(name string) {
	r.mu.Lock()
	r.opened[name] = true
	r.mu.Unlock()
}

func (r *readRecorder) wasOpened(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.opened[name]
}

func (r *readRecorder) Open(name string) (fs.File, error) {
	r.note(name)
	return r.FS.Open(name)
}

func (r *readRecorder) ReadFile(name string) ([]byte, error) {
	r.note(name)
	return fs.ReadFile(r.FS, name)
}

// Stat is not a read: TypeScript root discovery stats every tsconfig.json and
// package.json it passes, and a file it only measured must not be reported as
// one the analysis read from disk. Without this method fs.Stat would open the
// file through the recorder.
func (r *readRecorder) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(r.FS, name)
}

func (r *readRecorder) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(r.FS, name)
}
