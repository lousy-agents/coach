package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// ownedBase returns the cache root, creating it 0755 if needed. An existing
// root must be owned by this user and writable by no one else, because its
// copies and caches feed the tests: another user who could write there could
// plant modules or toolchains.
func ownedBase() (string, error) {
	base := os.Getenv("COACH_CI_ENV_CACHE")
	if base == "" {
		base = filepath.Join(os.TempDir(), "coach-ci-env-"+strconv.Itoa(os.Geteuid()))
	}
	info, err := os.Lstat(base)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", err
		}
		info, err = os.Lstat(base)
	}
	if err != nil {
		return "", err
	}
	if err := judgeBase(base, info); err != nil {
		return "", fmt.Errorf("%v; remove it or set COACH_CI_ENV_CACHE", err)
	}
	return base, nil
}

// judgeBase names the first reason the cache root cannot be trusted, if any.
func judgeBase(base string, info fs.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	switch {
	case !info.IsDir():
		return fmt.Errorf("the cache root %s is not a directory", base)
	case !ok || int(stat.Uid) != os.Geteuid():
		return fmt.Errorf("the cache root %s is owned by another user than uid %d", base, os.Geteuid())
	case info.Mode().Perm()&0o022 != 0:
		return fmt.Errorf("the cache root %s is writable by its group or by others", base)
	case os.Geteuid() == 0 && !reachableByOthers(base):
		return fmt.Errorf("the cache root %s cannot be reached by the non-root user the tests run as", base)
	}
	return nil
}
