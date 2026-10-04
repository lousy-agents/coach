package codesignalcli

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"sort"
)

func (sd *snapshotDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		remaining := sd.entries[sd.offset:]
		sd.offset = len(sd.entries)
		return remaining, nil
	}
	if sd.offset >= len(sd.entries) {
		return nil, io.EOF
	}
	end := sd.offset + n
	if end > len(sd.entries) {
		end = len(sd.entries)
	}
	batch := sd.entries[sd.offset:end]
	sd.offset = end
	return batch, nil
}

// normalizeSnapshotName validates a caller-supplied fs.FS name per the
// io/fs contract (slash-separated, no ./.. elements); "." denotes the
// snapshot root.
func normalizeSnapshotName(name string) (string, error) {
	if name == "." {
		return ".", nil
	}
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	return name, nil
}

// sanitizedSnapshotGitEnv returns the minimal environment for every git
// child a snapshot read spawns. It never forwards the parent process's
// ambient environment wholesale: only PATH (so the git executable can be
// found) and HOME are carried through, plus GIT_TERMINAL_PROMPT/
// GIT_CONFIG_NOSYSTEM to force non-interactive, system-config-free
// behavior, and GIT_NO_LAZY_FETCH to disable partial-clone promisor fetches
// so a blobless snapshot can never reach the network. Go-tooling and
// proxy/credential variables (GOPROXY, GOFLAGS,
// GOPATH, GO111MODULE, GONOSUMCHECK, GOSUMDB, HTTP(S)_PROXY, NO_PROXY, ...)
// are deliberately never forwarded, even when set in the parent process:
// this package only runs `git ls-tree`/`git show` against a local
// repository, which needs none of them, and forwarding them would be an
// unnecessary vector for those settings to influence a read that must stay
// local and hermetic.
func sanitizedSnapshotGitEnv() []string {
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_NO_LAZY_FETCH=1",
	}
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}
func finalizeSnapshotChildren(childSets map[string]map[string]bool) map[string][]fs.DirEntry {
	out := make(map[string][]fs.DirEntry, len(childSets))
	for dir, names := range childSets {
		entries := make([]fs.DirEntry, 0, len(names))
		for name, isDir := range names {
			entries = append(entries, snapshotDirEntry{name: name, isDir: isDir})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		out[dir] = entries
	}
	return out
}
func (sf *snapshotFile) Read(b []byte) (int, error) {
	if sf.pos >= len(sf.data) {
		return 0, io.EOF
	}
	n := copy(b, sf.data[sf.pos:])
	sf.pos += n
	return n, nil
}
