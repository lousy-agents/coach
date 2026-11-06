package gitrepo

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// FetchAdvice is how to bring in the objects a read of revision found missing from a partial clone, to be
// run in the user's own shell where git may fetch, or "" when the repository is not a partial clone or a
// name cannot be pasted safely. Coach never fetches, so the sentence is the way out.
//
// The batch lists the objects of that revision's tree and nothing else. `--no-walk` is what keeps it so:
// without it rev-list walks the whole history and the fetch would bring in every blob that ever was, which
// is the cost a partial clone exists to avoid. With a path, the single object the read wanted is offered
// first, since that is all a check that reads one file needs.
func FetchAdvice(run Runner, dir, revision, path string) string {
	remote, ok := PromisorRemote(run, dir)
	if !ok || !shellword.Printable(revision) || !shellword.Printable(remote) {
		return ""
	}
	batch := fmt.Sprintf("git rev-list --objects --missing=print --no-walk %s | sed -n 's/^?//p' | git fetch %s --no-tags --no-write-fetch-head --filter=blob:none --stdin", shellword.Word(revision), shellword.Word(remote))
	if path == "" || !shellword.Printable(path) {
		return "fetch the revision's files with " + batch + " and run again"
	}
	return "fetch it with git cat-file -e " + shellword.Quote(revision+":"+path) + ", or the revision's files with " + batch + ", and run again"
}
