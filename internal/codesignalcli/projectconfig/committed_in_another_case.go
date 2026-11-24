package projectconfig

import (
	"bytes"
	"path"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// committedInAnotherCase is the spelling at which revision commits the default policy only in another case, as
// .lousy-agents/Coach.json, or "" when it commits none. Such a policy is the one the revision holds for the default
// path: where the disk lacks it, it is restored at that spelling, since a new policy at the default spelling would
// stand beside it in the index. Of several spellings, the first in byte order is named.
func committedInAnotherCase(run gitrepo.Runner, dir, revision string) (string, error) {
	directory, file := path.Split(CanonicalPath)
	directories, err := treeEntriesFoldedTo(run, dir, revision, "", strings.TrimSuffix(directory, "/"), "tree")
	if err != nil {
		return "", err
	}
	for _, candidate := range directories {
		files, err := treeEntriesFoldedTo(run, dir, revision, candidate+"/", file, "blob")
		if err != nil {
			return "", err
		}
		if other := slices.DeleteFunc(files, func(spelling string) bool { return spelling == CanonicalPath }); len(other) > 0 {
			return other[0], nil
		}
	}
	return "", nil
}

// treeEntriesFoldedTo is the paths of the entries of kind directly under prefix at revision whose name equals name
// case-folded, sorted.
func treeEntriesFoldedTo(run gitrepo.Runner, dir, revision, prefix, name, kind string) ([]string, error) {
	args := []string{"ls-tree", "-z", "--full-tree", revision}
	if prefix != "" {
		args = append(args, "--", ":(top,literal)"+prefix)
	}
	output, err := run(dir, args...)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, record := range bytes.Split(output, []byte{0}) {
		if entry, ok := treeEntryOfKind(string(record), kind); ok && strings.EqualFold(path.Base(entry), name) {
			found = append(found, entry)
		}
	}
	slices.Sort(found)
	return found, nil
}

// treeEntryOfKind is the path of one ls-tree record ("<mode> <type> <object>\t<path>") when its type is kind.
func treeEntryOfKind(record, kind string) (string, bool) {
	meta, entry, ok := strings.Cut(record, "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 3 || fields[1] != kind {
		return "", false
	}
	return entry, true
}
