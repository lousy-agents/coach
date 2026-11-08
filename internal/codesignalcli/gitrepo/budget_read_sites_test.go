package gitrepo

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"testing"
	"time"
)

// git_budgets.go holds the numbers of the reads that name a budget of their own, and says that a package
// aliases them (projectconfig.MaxBytes is CommittedFileBytes). TestTheNamedReadsHoldTheirNumbersInOnePlace
// pins the numbers. This pins what each read passes: the constants are only worth their numbers if the read
// is built from the right ones, and an alias that names the wrong constant (a status held to the 1 MiB of the
// setup residue) fails nothing else.

// budgetArgument is one argument of a call, and the constant it must name. A name that is not a constant of
// the table (the listing's wall time) is compared as it is written at the call.
type budgetArgument struct {
	index  int
	want   string
	number int64
	asHeld bool
}

// readSite is the one call in a file that holds a read to its limits.
type readSite struct {
	name string
	dir  string
	file string
	call string
	args []budgetArgument
}

// namedBudgets is the value each constant of git_budgets.go that a read is built from has now; durations are
// nanoseconds.
var namedBudgets = map[string]int64{
	"WorktreeStatusBytes": WorktreeStatusBytes, "GitStderrBytes": GitStderrBytes, "WorktreeStatusTimeout": int64(WorktreeStatusTimeout),
	"WorktreeRootBytes": WorktreeRootBytes, "WorktreeRootStderr": WorktreeRootStderr, "WorktreeRootTimeout": int64(WorktreeRootTimeout),
	"CommittedFileBytes": CommittedFileBytes, "CommittedFileTimeout": int64(CommittedFileTimeout),
	"SetupResidueBytes": SetupResidueBytes, "SetupResidueTimeout": int64(SetupResidueTimeout),
}

var readSites = []readSite{
	{"git status", ".", "worktree_status.go", "BoundedRunner", []budgetArgument{{0, "WorktreeStatusBytes", 16 << 20, false}, {1, "GitStderrBytes", 64 << 10, false}, {2, "WorktreeStatusTimeout", int64(30 * time.Second), false}}},
	{"the worktree root of the compiler", "../tstoolchain", "compiler_resolution.go", "RunBytesBounded", []budgetArgument{{1, "WorktreeRootBytes", 4 << 10, false}, {2, "WorktreeRootStderr", 4 << 10, false}, {3, "WorktreeRootTimeout", int64(10 * time.Second), false}}},
	{"a file of the package manager at a revision", "../pkgmanager", "revision_detection.go", "BoundedRunner", []budgetArgument{{0, "CommittedFileBytes", 1 << 20, false}, {1, "GitStderrBytes", 64 << 10, false}, {2, "CommittedFileTimeout", int64(30 * time.Second), false}}},
	{"the policy document at a revision", "../projectconfig", "git_runner.go", "BoundedRunner", []budgetArgument{{0, "CommittedFileBytes", 1 << 20, false}, {1, "GitStderrBytes", 64 << 10, false}, {2, "CommittedFileTimeout", int64(30 * time.Second), false}}},
	{"the status of the setup residue", "../tssetup", "residue.go", "BoundedRunner", []budgetArgument{{0, "SetupResidueBytes", 1 << 20, false}, {1, "GitStderrBytes", 64 << 10, false}, {2, "SetupResidueTimeout", int64(10 * time.Second), false}}},
	{"the listing of the tree for the snapshot", ".", "snapshot_entries.go", "streamListingBy", []budgetArgument{{2, "snapshotListingWallTime", 0, true}}},
	{"the listing of the tracked files", ".", "tracked_files.go", "streamListing", []budgetArgument{{1, "snapshotListingWallTime", 0, true}}},
}

// check fails the test for each argument of the site's call that does not name the constant it must.
func (site readSite) check(t *testing.T) {
	calls := callsOf(parseFile(t, filepath.Join(site.dir, site.file)), site.call)
	if len(calls) != 1 {
		t.Fatalf("%s: %d calls of %s in %s, want the one that holds the read to its limits", site.name, len(calls), site.call, site.file)
	}
	aliases := aliasesOf(t, site.dir)
	for _, argument := range site.args {
		got := site.passes(t, calls[0], argument, aliases)
		if got != argument.want {
			t.Errorf("%s: argument %d of %s names %s, want %s", site.name, argument.index, site.call, got, argument.want)
		} else if number, named := namedBudgets[got]; named && number != argument.number {
			t.Errorf("%s: %s is %d, want %d", site.name, got, number, argument.number)
		}
	}
}

func (site readSite) passes(t *testing.T, call *ast.CallExpr, argument budgetArgument, aliases map[string]ast.Expr) string {
	t.Helper()
	if argument.index >= len(call.Args) {
		return fmt.Sprintf("no argument %d", argument.index)
	}
	expression := call.Args[argument.index]
	if argument.asHeld {
		return types.ExprString(expression)
	}
	name, err := namedBudget(expression, aliases, site.dir == ".", 0)
	if err != nil {
		return err.Error()
	}
	return name
}

func TestEveryReadThatNamesABudgetIsBuiltFromTheConstantsOfItsRow(t *testing.T) {
	for _, site := range readSites {
		t.Run(site.name, site.check)
	}
}

// The listing of the tree has no limit on its size, and the wall time that bounds it is two minutes.
func TestTheListingOfTheTreeIsHeldToTwoMinutes(t *testing.T) {
	if snapshotListingWallTime != 2*time.Minute {
		t.Errorf("snapshotListingWallTime = %s, want 2m0s, the wall time of the list of changed files", snapshotListingWallTime)
	}
}
