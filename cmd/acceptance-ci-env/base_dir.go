package main

import (
	"fmt"
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
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	info, err := os.Lstat(base)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || int(stat.Uid) != os.Geteuid() || info.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("the cache root %s must be a directory owned by uid %d and writable by no one else; remove it or set COACH_CI_ENV_CACHE", base, os.Geteuid())
	}
	if os.Geteuid() == 0 && !reachableByOthers(base) {
		return "", fmt.Errorf("the cache root %s cannot be reached by the non-root user the tests run as; set COACH_CI_ENV_CACHE to a directory every user can search", base)
	}
	return base, nil
}
