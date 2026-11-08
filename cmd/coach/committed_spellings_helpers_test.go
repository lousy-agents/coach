package main

import (
	"fmt"
	"sort"
	"strings"
)

// folder is a directory of a tree built for the plumbing: a name maps to a blob id, or to a folder.
type folder map[string]any

// commitPolicyUnderSpellings commits, over the repository's commit, a tree that holds the policy under each of the
// spellings given. The plumbing writes them as asked, which no file system that folds case could check out together,
// and it leaves the working tree and the index alone, so what the revision holds is the only thing that differs
// between disks.
func commitPolicyUnderSpellings(repo string, spellings ...string) {
	policy := plumb(repo, singleRootPolicyJSON, "hash-object", "-w", "--stdin")
	source := plumb(repo, minimalGoSource, "hash-object", "-w", "--stdin")
	root := folder{"a.go": source}
	for _, spelling := range spellings {
		place(root, strings.Split(spelling, "/"), policy)
	}
	tree := writeFolder(repo, root)
	plumb(repo, "", "update-ref", "HEAD", plumb(repo, "", "commit-tree", tree, "-m", "the policy under "+strings.Join(spellings, ", ")))
}

func place(directory folder, parts []string, blob string) {
	if len(parts) == 1 {
		directory[parts[0]] = blob
		return
	}
	inner, ok := directory[parts[0]].(folder)
	if !ok {
		inner = folder{}
		directory[parts[0]] = inner
	}
	place(inner, parts[1:], blob)
}

func writeFolder(repo string, directory folder) string {
	names := make([]string, 0, len(directory))
	for name := range directory {
		names = append(names, name)
	}
	sort.Strings(names)
	var listing strings.Builder
	for _, name := range names {
		switch entry := directory[name].(type) {
		case folder:
			fmt.Fprintf(&listing, "040000 tree %s\t%s\n", writeFolder(repo, entry), name)
		case string:
			fmt.Fprintf(&listing, "100644 blob %s\t%s\n", entry, name)
		}
	}
	return plumb(repo, listing.String(), "mktree")
}
