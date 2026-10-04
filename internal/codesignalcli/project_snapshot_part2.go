package codesignalcli

import (
	"fmt"

	"io/fs"

	"path"

	"strconv"
	"strings"
)

func (f *goSnapshotFS) Open(name string) (fs.File, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return &snapshotDirFile{name: clean, entries: f.children[clean]}, nil
	}
	if f.isFile[clean] {
		data, err := f.readFile(clean)
		if err != nil {
			return nil, err
		}
		return &snapshotFile{name: clean, data: data}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
func (s *snapshotPaths) add(filePath string) {
	segments := strings.Split(filePath, "/")
	parent := "."
	for i, segment := range segments {
		if s.childSets[parent] == nil {
			s.childSets[parent] = map[string]bool{}
		}
		if i == len(segments)-1 {
			s.childSets[parent][segment] = false
			return
		}
		s.childSets[parent][segment] = true
		parent = path.Join(parent, segment)
		s.isDir[parent] = true
	}
}

// parseSnapshotLsTreeEntry parses one `git ls-tree -l` entry (already split
// on the `-z` NUL terminator) into its path and blob size. The long format
// is "<mode> SP <type> SP <object> SP <size> TAB <path>"; size is
// whitespace-padded, not tab-separated from the preceding fields, so the
// metadata is split by field first and the path is taken verbatim after the
// first tab (a path may itself contain spaces). git reports a size it could
// not determine as a non-numeric sentinel: "-" for a non-blob entry (e.g. a
// submodule gitlink), and the literal "BAD" for a blob whose object is
// missing or corrupt from the local object store. Both are parsed here as
// size 0 rather than rejected: this construction step must not be the place
// a missing blob surfaces as a failure, since erroring the whole listing
// here would discard the specific path the caller needs to report -- the
// existing per-path failure instead surfaces naturally, with that path
// intact, the moment something actually tries to read the blob's content.
func parseSnapshotLsTreeEntry(entry string) (path string, size int64, err error) {
	meta, p, found := strings.Cut(entry, "\t")
	if !found {
		return "", 0, fmt.Errorf("missing tab-separated path")
	}
	fields := strings.Fields(meta)
	if len(fields) < 4 {
		return "", 0, fmt.Errorf("expected mode/type/object/size, got %q", meta)
	}
	parsedSize, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return p, 0, nil
	}
	return p, parsedSize, nil
}
