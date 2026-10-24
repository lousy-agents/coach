package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func copyWorktreeProjectConfig(dir string) error {
	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, projectconfig.DefaultPath))
	if err != nil {
		return err
	}
	destDir := filepath.Join(root, filepath.Dir(projectconfig.CanonicalPath))
	dest := filepath.Join(root, projectconfig.CanonicalPath)
	createdDir, err := prepareMigrationDirectory(destDir)
	if err != nil {
		return err
	}
	if err := writeMigrationFile(dest, data); err != nil {
		removeFailedMigration(destDir, createdDir)
		return err
	}
	return nil
}

func prepareMigrationDirectory(destDir string) (bool, error) {
	mkdirErr := os.Mkdir(destDir, 0o755)
	if mkdirErr == nil {
		return true, nil
	}
	info, statErr := os.Lstat(destDir)
	if statErr != nil {
		return false, mkdirErr
	}
	// Lstat so a symlink to a directory is rejected instead of followed.
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("parent path component %q is a symlink", filepath.Base(destDir))
	}
	if !info.IsDir() {
		return false, mkdirErr
	}
	return false, nil
}

func writeMigrationFile(dest string, data []byte) error {
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(dest)
	}
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func removeFailedMigration(destDir string, createdDir bool) {
	if !createdDir {
		return
	}
	entries, err := os.ReadDir(destDir)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(destDir)
}
