package projectconfig

import (
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var errListing = errors.New("input/output error")

// maxListingAttempts is how often a listing may be asked again after it failed. A check that asks more has
// swallowed the failure and is waiting for an end that a failing directory never sends.
const maxListingAttempts = 3

// failingListing answers every listing with errListing, and ends the test when the check keeps asking.
type failingListing struct {
	diskFile
	t        *testing.T
	attempts int
}

func (f *failingListing) Readdirnames(int) ([]string, error) {
	f.attempts++
	if f.attempts > maxListingAttempts {
		f.t.Fatalf("the directory was listed %d times after it failed: the failure was not reported", f.attempts)
	}
	return nil, errListing
}

// endingAfterAFailure fails the first listing and ends every later one, so a check that swallows the
// failure finishes and shows what it made of it.
type endingAfterAFailure struct {
	diskFile
	failed bool
}

func (f *endingAfterAFailure) Readdirnames(int) ([]string, error) {
	if f.failed {
		return nil, io.EOF
	}
	f.failed = true
	return nil, errListing
}

type diskWhoseListingFails struct {
	foldingDisk
	wrap func(diskFile) diskFile
}

func (d diskWhoseListingFails) OpenFile(name string, flag int, perm fs.FileMode) (diskFile, error) {
	file, err := d.foldingDisk.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return d.wrap(file), nil
}

func persistentlyFailing(t *testing.T) diskWhoseListingFails {
	return diskWhoseListingFails{foldingDiskHolding("cfg/p.json"), func(file diskFile) diskFile { return &failingListing{diskFile: file, t: t} }}
}

func failingOnce() diskWhoseListingFails {
	return diskWhoseListingFails{foldingDiskHolding("cfg/p.json"), func(file diskFile) diskFile { return &endingAfterAFailure{diskFile: file} }}
}

func failingDisks(t *testing.T) map[string]diskWhoseListingFails {
	return map[string]diskWhoseListingFails{"a listing that fails and then ends": failingOnce(), "a listing that always fails": persistentlyFailing(t)}
}

// A directory that cannot be listed leaves its entries' spelling unknown: that is an error, never "no entry"
// and never a spelling.
func TestAFailedListingOfADirectoryIsReportedNotTakenForAnEntryThatIsNotThere(t *testing.T) {
	for name, disk := range failingDisks(t) {
		t.Run(name, func(t *testing.T) {
			assertListingFailureReported(t, disk)
		})
	}
}

func assertListingFailureReported(t *testing.T, disk diskWhoseListingFails) {
	t.Helper()

	got, err := spellingIn(disk, "cfg", "p.json")

	if !errors.Is(err, errListing) {
		t.Errorf("spellingIn = %q, %v; want the listing's own failure", got, err)
	}
}

func TestAPathWhoseDirectoryCannotBeListedIsRefusedAsUnreadableNotAsSpelledInAnotherCase(t *testing.T) {
	for name, disk := range failingDisks(t) {
		t.Run(name, func(t *testing.T) {
			requireUnreadableOnDisk(t, checkExactNames(disk, "cfg/p.json"))
		})
	}
}

func TestTheParentsOfAPathWhoseDirectoryCannotBeListedAreRefusedTheSameWay(t *testing.T) {
	requireUnreadableOnDisk(t, checkExactParents(persistentlyFailing(t), "cfg/p.json"))
}

func requireUnreadableOnDisk(t *testing.T, err error) {
	t.Helper()
	found := configErrorOf(t, err)
	if found.PolicyCause != projectreadiness.CauseUnreadableOnDisk {
		t.Errorf("cause %q, %q; want %q", found.PolicyCause, found.Message, projectreadiness.CauseUnreadableOnDisk)
	}
}
