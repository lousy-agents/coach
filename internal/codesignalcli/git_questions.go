package codesignalcli

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// runGit asks git the per-file questions of a --base scan: the content of a file at a revision, and the
// changed lines of a file. It is a variable so that a test can answer them from a fake that is the whole of
// git for the test; production code never assigns it.
var runGit gitrepo.Runner = gitrepo.RunBytes
