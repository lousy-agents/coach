package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func goEnv(name string) (string, error) {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return "", fmt.Errorf("go env %s: %v", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// keepCallersGoCaches names the caller's own Go caches explicitly, because
// their defaults live under HOME, and the empty HOME must stay empty.
func keepCallersGoCaches(overrides map[string]string) error {
	for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		value, err := goEnv(name)
		if err != nil {
			return err
		}
		overrides[name] = value
	}
	return nil
}

// useNobodysGoCaches gives the non-root user caches it owns: a copy of the
// module cache, made once per go.sum so a new dependency gets a fresh copy, a
// build cache and a GOPATH. GOROOT is unset so go finds the copied toolchain.
func useNobodysGoCaches(base string, overrides map[string]string, uid, gid int) error {
	modcache, err := goEnv("GOMODCACHE")
	if err != nil {
		return err
	}
	sum, err := os.ReadFile("go.sum")
	if err != nil {
		return err
	}
	copied := filepath.Join(base, "gomodcache-"+hashOf(string(sum)))
	if err := copyOnce(modcache, copied, gid); err != nil {
		return err
	}
	for name, path := range map[string]string{"GOMODCACHE": copied, "GOCACHE": filepath.Join(base, "gocache"), "GOPATH": filepath.Join(base, "gopath")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := chownTree(path, uid, gid); err != nil {
			return err
		}
		overrides[name] = path
	}
	overrides["GOTOOLCHAIN"] = "local"
	overrides["GOROOT"] = unset
	return nil
}
