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

// callersGoCaches names the caller's own Go caches explicitly, because their
// defaults live under HOME, and the empty HOME must stay empty.
func callersGoCaches() (map[string]string, error) {
	env := map[string]string{}
	for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		value, err := goEnv(name)
		if err != nil {
			return nil, err
		}
		env[name] = value
	}
	return env, nil
}

// nobodysGoCaches gives the non-root user caches it owns: a copy of the module
// cache, made once per go.sum so a new dependency gets a fresh copy, a build
// cache and a GOPATH. GOROOT is unset so go finds the copied toolchain, and
// GOTOOLCHAIN is local so it does not fetch another one into a cache it may
// not write.
func nobodysGoCaches(base string, uid, gid int) (map[string]string, error) {
	modcache, err := goEnv("GOMODCACHE")
	if err != nil {
		return nil, err
	}
	sum, err := os.ReadFile("go.sum")
	if err != nil {
		return nil, err
	}
	copied := filepath.Join(base, "gomodcache-"+hashOf(string(sum)))
	if err := copyOnce(modcache, copied, gid); err != nil {
		return nil, err
	}
	env := map[string]string{"GOTOOLCHAIN": "local", "GOROOT": unset}
	for name, path := range map[string]string{"GOMODCACHE": copied, "GOCACHE": filepath.Join(base, "gocache"), "GOPATH": filepath.Join(base, "gopath")} {
		if err := ownedBy(path, uid, gid); err != nil {
			return nil, err
		}
		env[name] = path
	}
	return env, nil
}

func ownedBy(path string, uid, gid int) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return chownTree(path, uid, gid)
}
