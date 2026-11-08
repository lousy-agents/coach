package projectconfig

import (
	"crypto/sha1" //nolint:gosec // git names a blob by this hash; it is not used for security here.
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
)

// maxComparedBytes is the largest file compared with the committed copy. A file past it is not read, and is taken to
// differ: a policy that large is not one a replacement is safe to offer over.
const maxComparedBytes = 64 << 20

// fileHoldsTheCommittedBlob is whether the regular file at repoPath under the worktree rooted at rootDir holds exactly
// the bytes of the committed blob named objectID, hashed as git hashes a blob. It reads the file through the root and
// asks git nothing, so no index is refreshed or written, no hook runs, no filter converts the bytes, and a skip-worktree
// or assume-unchanged mark, which git takes on trust, cannot make an edited file look unchanged. Anything that cannot
// be decided (a link or another entry that is not a regular file, a file that cannot be read, one that changes while it
// is read, one that is too large, an object id of a length no hash is known for) is not the same, because a replacement
// offered over a file that was not understood can destroy what is in it. The cost is that a checkout whose line-ending or
// smudge conversion makes the bytes differ from the blob's is told apart from it: that is the safe side.
func fileHoldsTheCommittedBlob(rootDir, repoPath, objectID string) bool {
	blob := blobHash(objectID)
	if blob == nil {
		return false
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	name := filepath.FromSlash(repoPath)
	if info, err := root.Lstat(name); err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := root.OpenFile(name, os.O_RDONLY|openWithoutWaiting, 0)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	return hashesTo(file, blob, objectID)
}

// hashesTo is whether file, a regular file, hashes as the blob objectID names.
func hashesTo(file *os.File, blob hash.Hash, objectID string) bool {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxComparedBytes {
		return false
	}
	_, _ = fmt.Fprintf(blob, "blob %d\x00", info.Size())
	read, err := io.Copy(blob, io.LimitReader(file, info.Size()+1))
	return err == nil && read == info.Size() && hex.EncodeToString(blob.Sum(nil)) == objectID
}

// blobHash is the hash git names blobs with in a repository whose object ids are as long as objectID, or nil.
func blobHash(objectID string) hash.Hash {
	switch len(objectID) {
	case 40:
		return sha1.New() //nolint:gosec // see above.
	case 64:
		return sha256.New()
	}
	return nil
}
