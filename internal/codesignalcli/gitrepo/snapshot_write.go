package gitrepo

import "os"

// writeSnapshot writes the entries below dir. Each blob is read once and written to every path that names
// it. A submodule is a commit of another repository, so it becomes an empty directory. Every write goes
// through one os.Root, so a tree that names a path outside dir, or through a link, is an error and not a
// file written elsewhere.
func writeSnapshot(repositoryRoot, dir string, entries []snapshotEntry) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	blobs, err := makeSubmoduleDirectories(root, entries)
	if err != nil {
		return err
	}
	order, byObject := groupByObject(blobs)
	missing, err := readObjects(repositoryRoot, order, func(oid string, content []byte) error {
		return writeEveryPathOf(root, byObject[oid], content)
	})
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return &ObjectsNotLocalError{Count: len(missing), FirstPath: byObject[missing[0]][0].path}
	}
	return nil
}

func writeEveryPathOf(root *os.Root, entries []snapshotEntry, content []byte) error {
	for _, entry := range entries {
		if err := writeSnapshotEntry(root, entry, content); err != nil {
			return err
		}
	}
	return nil
}
