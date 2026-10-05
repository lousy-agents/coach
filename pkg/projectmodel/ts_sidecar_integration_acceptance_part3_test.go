package projectmodel_test

import (
	"encoding/json"

	"io/fs"

	"path/filepath"

	"testing/fstest"

	. "github.com/onsi/gomega"
)

// copyDirRecursive copies every file and directory under src into dst,
// used by the coverage-identity spec to mutate a private copy of the
// vendored sidecar tree without touching the real build output.
func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		return body_tsSidecarIntegrationAcceptancePart3Test_20(p, d, err, src, dst)
	})
}

func tsconfigJSON(v any) *fstest.MapFile {
	data, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	return &fstest.MapFile{Data: data}
}
