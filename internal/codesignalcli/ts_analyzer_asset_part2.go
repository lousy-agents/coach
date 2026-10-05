package codesignalcli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
)

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
