package revisionfs

import (
	"fmt"
	"io/fs"
)

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
