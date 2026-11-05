// Package worktreeoverlay serves a committed snapshot with the uncommitted
// state of chosen working-tree paths laid over it, so project analysis reads
// what is on disk for exactly those paths and committed bytes for the rest.
package worktreeoverlay

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Outcome says what the overlay did with each path it was given.
type Outcome struct {
	// Overlaid paths are served from disk: modified, added or untracked files.
	Overlaid []string
	// Removed paths were tracked and are gone from disk, so they are absent.
	Removed []string
	// SymlinkSkipped paths are symbolic links on disk. They are absent: the
	// link is not followed and the committed bytes are not served for it.
	SymlinkSkipped []string
	// Unreadable paths are absent and say why, one reason each.
	Unreadable []Unreadable
	// Changed paths were modified on disk while they were being read, so no
	// consistent bytes exist for them. They are absent.
	Changed []string
}

// Unreadable is a path the overlay could not read, with the reason.
type Unreadable struct {
	Path   string
	Reason string
}

// Reasons an Unreadable path could not be read.
const (
	ReasonNonRegular       = "non_regular"
	ReasonTooLarge         = "too_large"
	ReasonPermissionDenied = "permission_denied"
	ReasonReadFailed       = "read_failed"
	ReasonNameMismatch     = "name_mismatch"
)

// New returns base with the disk state of paths laid over it. Each path's
// state is decided from disk alone, so staged and unstaged edits behave the
// same and a rename is a removed path plus an added one. Every overlaid file is
// read once, here, so later reads are consistent.
// Option adjusts how New reads the disk.
type Option func(*settings)

type settings struct{ caseInsensitive bool }

// CaseInsensitiveNames makes New require each path's exact spelling in its
// directory listing, because on a case-insensitive filesystem Lstat also
// succeeds for the old spelling of a case-only rename. Each directory is listed
// once per build.
func CaseInsensitiveNames() Option {
	return func(s *settings) { s.caseInsensitive = true }
}

func New(base fs.FS, paths []string, disk DiskSource, options ...Option) (fs.FS, Outcome, error) {
	cleaned, err := cleanPaths(paths)
	if err != nil {
		return nil, Outcome{}, err
	}
	var config settings
	for _, option := range options {
		option(&config)
	}
	if config.caseInsensitive {
		disk = &exactNameDisk{DiskSource: disk, listings: map[string]map[string]bool{}, twins: newCaseTwins(cleaned)}
	}
	overlay := &overlayFS{
		base:   base,
		files:  map[string][]byte{},
		hidden: map[string]bool{},
	}
	build := overlayBuild{overlay: overlay}
	for _, name := range cleaned {
		kind, data := readDisk(disk, name)
		build.add(name, kind, data, tracked(base, name))
	}
	overlay.index()
	return overlay, build.outcome, nil
}

// overlayBuild owns the overlay and the Outcome it is filling in, so each path
// updates both through one receiver.
type overlayBuild struct {
	overlay *overlayFS
	outcome Outcome
}

func (b *overlayBuild) add(name string, kind diskKind, data []byte, inBase bool) {
	switch kind {
	case diskRegular:
		b.overlay.files[name] = data
		b.outcome.Overlaid = append(b.outcome.Overlaid, name)
	case diskMissing:
		if inBase {
			b.overlay.hidden[name] = true
			b.outcome.Removed = append(b.outcome.Removed, name)
		}
	case diskSymlink:
		b.overlay.hidden[name] = true
		b.outcome.SymlinkSkipped = append(b.outcome.SymlinkSkipped, name)
	case diskChanged:
		b.overlay.hidden[name] = true
		b.outcome.Changed = append(b.outcome.Changed, name)
	default:
		b.overlay.hidden[name] = true
		b.outcome.Unreadable = append(b.outcome.Unreadable, Unreadable{Path: name, Reason: unreadableReason(kind)})
	}
}

func unreadableReason(kind diskKind) string {
	switch kind {
	case diskNonRegular:
		return ReasonNonRegular
	case diskTooLarge:
		return ReasonTooLarge
	case diskPermissionDenied:
		return ReasonPermissionDenied
	case diskNameMismatch:
		return ReasonNameMismatch
	}
	return ReasonReadFailed
}

func tracked(base fs.FS, name string) bool {
	info, err := fs.Stat(base, name)
	return err == nil && !info.IsDir()
}

func cleanPaths(paths []string) ([]string, error) {
	seen := make(map[string]bool, len(paths))
	cleaned := make([]string, 0, len(paths))
	for _, name := range paths {
		if isDirectoryMarker(name) {
			continue
		}
		if name == "." || !fs.ValidPath(name) {
			return nil, fmt.Errorf("coach: worktree overlay path %q is not a normalized repository-relative path", name)
		}
		if !seen[name] {
			seen[name] = true
			cleaned = append(cleaned, name)
		}
	}
	sort.Strings(cleaned)
	return cleaned, nil
}

// isDirectoryMarker reports whether name is a directory as git status writes
// one: a well-formed path with a trailing slash, such as the untracked nested
// repository it lists without entering. A directory holds no bytes to overlay.
func isDirectoryMarker(name string) bool {
	directory, marked := strings.CutSuffix(name, "/")
	return marked && directory != "." && fs.ValidPath(directory)
}
