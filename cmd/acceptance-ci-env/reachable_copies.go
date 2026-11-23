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
func reachablePath(base, path, gitDir string, gid int) (string, error) {
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
		copied, err := cachedCopy(base, entry, gid)
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

// cachedCopy copies a PATH entry once into the cache. A bin directory inside a
// mise install is copied with its install, because a toolchain such as Go
// finds its root relative to its binary; any other entry is copied alone, so
// a bin such as ~/.local/bin does not bring all of ~/.local with it.
func cachedCopy(base, entry string, gid int) (string, error) {
	source, suffix := entry, ""
	if filepath.Base(entry) == "bin" && strings.Contains(entry, string(filepath.Separator)+"installs"+string(filepath.Separator)) {
		source, suffix = filepath.Dir(entry), "bin"
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}
	target := filepath.Join(base, "tools", hashOf(source))
	if err := copyOnce(source, target, gid); err != nil {
		return "", err
	}
	return filepath.Join(target, suffix), nil
}

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
