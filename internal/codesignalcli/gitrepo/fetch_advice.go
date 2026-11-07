package gitrepo

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// pasteableAsAnArgument is whether word can stand where git reads an argument that is not an option. Quoting
// does not make a word that starts with a dash an argument: the shell removes the quotes, and git reads
// `--upload-pack=<command>` as the option it spells, which runs the command.
func pasteableAsAnArgument(word string) bool {
	return shellword.Printable(word) && !strings.HasPrefix(word, "-")
}

// FetchAdvice is how to bring in the objects a read of revision found missing from a partial clone, to be
// run in the user's own shell where git may fetch, or "" when the repository is not a partial clone or a
// name cannot be pasted safely: one with a control character, or one that git would read as an option. Coach never fetches, so the sentence is the way out.
//
// The batch lists the objects of that revision's tree and nothing else. `--no-walk` is what keeps it so:
// without it rev-list walks the whole history and the fetch would bring in every blob that ever was, which
// is the cost a partial clone exists to avoid. With a path, the single object the read wanted is offered
// first, since that is all a check that reads one file needs.
func FetchAdvice(run Runner, dir, revision, path string) string {
	return fetchAdviceFor(run, dir, []string{revision}, path)
}

// FetchAdviceFor is FetchAdvice for every revision a scan reads, in one batch, so that one paste brings in
// what the whole scan needs and not what the first of its reads found missing.
func FetchAdviceFor(run Runner, dir string, revisions ...string) string {
	return fetchAdviceFor(run, dir, revisions, "")
}

func fetchAdviceFor(run Runner, dir string, revisions []string, path string) string {
	remote, ok := PromisorRemote(run, dir)
	if !ok || !pasteableAsAnArgument(remote) || slices.ContainsFunc(revisions, func(r string) bool { return !pasteableAsAnArgument(r) }) {
		return ""
	}
	batch := fetchBatch(remote, revisions)
	if path == "" || !shellword.Printable(path) {
		return "fetch the revision's files with " + batch + " and run again"
	}
	return "fetch it with git cat-file -e " + shellword.Quote(revisions[0]+":"+path) + ", or the revision's files with " + batch + ", and run again"
}

// maxFetchPasses bounds the batch. A clone made without trees gets the trees of a revision in one pass, and
// only then can it list the blobs those trees name, which the next pass brings; a pass that finds nothing
// missing ends the batch, so this bound is a ceiling on a fetch that brings nothing new, not a count.
const maxFetchPasses = 4

// fetchBatch lists what the revisions lack and fetches it, again until nothing is missing: the listing of a
// revision whose trees are missing names only the trees, and what they name can be listed once they are here.
// A fetch that fails ends it, and git has said why. An empty list is never handed to git fetch, which would
// read its refspecs from the remote's configuration and update refs.
func fetchBatch(remote string, revisions []string) string {
	listing := ""
	for _, revision := range revisions {
		listing += "git rev-list --objects --missing=print --no-walk " + shellword.Word(revision) + "; "
	}
	if len(revisions) == 1 {
		listing = strings.TrimSuffix(listing, "; ")
	} else {
		listing = "{ " + listing + "}"
	}
	passes := make([]string, maxFetchPasses)
	for i := range passes {
		passes[i] = strconv.Itoa(i + 1)
	}
	return fmt.Sprintf("for pass in %s; do ids=$(%s | sed -n 's/^?//p'); [ -n \"$ids\" ] || break; printf '%%s\\n' \"$ids\" | git fetch %s --no-tags --no-write-fetch-head --filter=blob:none --stdin || break; done", strings.Join(passes, " "), listing, shellword.Word(remote))
}
