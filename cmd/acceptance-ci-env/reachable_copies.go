package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func nobody() (uid, gid int, err error) {
	account, err := user.Lookup("nobody")
	if err != nil {
		return 0, 0, fmt.Errorf("running as root needs a non-root user to run the tests as, and there is no `nobody` user: %v", err)
	}
	uid, _ = strconv.Atoi(account.Uid)
	gid, _ = strconv.Atoi(account.Gid)
	return uid, gid, nil
}

// reachableByOthers reports whether a user that is neither the owner nor in
// the group can reach dir: every directory from the root down must be
// searchable by others.
func reachableByOthers(dir string) bool {
	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err != nil || info.Mode().Perm()&0o001 == 0 {
			return false
		}
		if current == filepath.Dir(current) {
			return true
		}
	}
}

// reachablePath keeps every PATH entry `nobody` can reach, copies the
// unreachable ones under the user's own tool installs (mise and ~/.local) into
// the cache, and drops the rest, as CI's PATH has none of them. The git
// directory goes first, so the git that was checked is the git that runs.
func reachablePath(base, path, gitDir string) (string, error) {
	entries := append([]string{gitDir}, filepath.SplitList(path)...)
	var kept []string
	for _, entry := range entries {
		if reachableByOthers(entry) {
			kept = append(kept, entry)
			continue
		}
		if !isToolInstall(entry) && entry != gitDir {
			continue
		}
		copied, err := cachedCopy(base, entry)
		if err != nil {
			return "", err
		}
		kept = append(kept, copied)
	}
	return strings.Join(kept, string(filepath.ListSeparator)), nil
}

func isToolInstall(entry string) bool {
	home, _ := os.UserHomeDir()
	return home != "" && strings.HasPrefix(entry, filepath.Join(home, ".local")+string(filepath.Separator))
}

// cachedCopy copies a PATH entry once into the cache. An entry named bin is
// copied with its parent, because a toolchain such as Go finds its root
// relative to its binary.
func cachedCopy(base, entry string) (string, error) {
	source, suffix := entry, ""
	if filepath.Base(entry) == "bin" {
		source, suffix = filepath.Dir(entry), "bin"
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}
	sum := sha256.Sum256([]byte(source))
	target := filepath.Join(base, "tools", hex.EncodeToString(sum[:8]))
	if err := copyOnce(source, target); err != nil {
		return "", err
	}
	return filepath.Join(target, suffix), nil
}

// copyOnce copies source to target unless an earlier run finished the same
// copy, which leaves a marker beside it.
func copyOnce(source, target string) error {
	marker := target + ".done"
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	_ = os.RemoveAll(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if out, err := exec.Command("cp", "-a", source, target).CombinedOutput(); err != nil {
		return fmt.Errorf("copying %s for the non-root user: %v: %s", source, err, out)
	}
	if out, err := exec.Command("chmod", "-R", "a+rX", target).CombinedOutput(); err != nil {
		return fmt.Errorf("opening %s to the non-root user: %v: %s", target, err, out)
	}
	return os.WriteFile(marker, nil, 0o644)
}

// useReachableGoCaches gives the non-root user a module cache it can read and
// a build cache it can write.
func (w *workspace) useReachableGoCaches(base string, overrides map[string]string) error {
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return fmt.Errorf("go env GOMODCACHE: %v", err)
	}
	modcache := strings.TrimSpace(string(out))
	if !reachableByOthers(modcache) {
		target := filepath.Join(base, "gomodcache")
		if err := copyOnce(modcache, target); err != nil {
			return err
		}
		modcache = target
	}
	gocache := filepath.Join(base, "gocache")
	if err := os.MkdirAll(gocache, 0o777); err != nil {
		return err
	}
	if err := os.Chmod(gocache, 0o777); err != nil {
		return err
	}
	gopath := filepath.Join(base, "gopath")
	if err := os.MkdirAll(gopath, 0o777); err != nil {
		return err
	}
	if err := os.Chmod(gopath, 0o777); err != nil {
		return err
	}
	overrides["GOMODCACHE"] = modcache
	overrides["GOCACHE"] = gocache
	overrides["GOPATH"] = gopath
	overrides["GOTOOLCHAIN"] = "local"
	overrides["GOROOT"] = unset
	return nil
}

func chownTree(dir string, uid, gid int) error {
	return filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, uid, gid)
	})
}
