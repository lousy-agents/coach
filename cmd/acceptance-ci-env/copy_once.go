package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

// copyOnce copies source to target unless an earlier run finished the same
// copy, which leaves a marker beside it. The copy is readable by the given
// group, the non-root user's, and by no one else.
func copyOnce(source, target string, gid int) error {
	marker := target + ".done"
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	_ = os.RemoveAll(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	steps := [][]string{
		{"cp", "-a", source, target},
		{"chgrp", "-R", strconv.Itoa(gid), target},
		{"chmod", "-R", "g+rX,o-rwx", target},
	}
	for _, step := range steps {
		if out, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("preparing %s for the non-root user (%s): %v: %s", source, step[0], err, out)
		}
	}
	return os.WriteFile(marker, nil, 0o644)
}

func chownTree(dir string, uid, gid int) error {
	return filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, uid, gid)
	})
}
