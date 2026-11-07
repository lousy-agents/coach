package projectconfig

import (
	"errors"
	"syscall"
	"testing"
)

// openDirectory lets the handle, not the stat that found the entry, decide what it is: a name that was a directory
// when it was listed can be something else by the time it is opened, and a handle that is not a directory is refused
// with ENOTDIR and closed.
func TestAHandleThatIsNotADirectoryIsRefusedAsOne(t *testing.T) {
	disk := foldingDiskHolding("cfg")

	listing, err := openDirectory(disk, "cfg")

	if !errors.Is(err, syscall.ENOTDIR) || listing != nil {
		t.Errorf("openDirectory = %v, %v; want ENOTDIR and no handle for a file", listing, err)
	}
}

func TestADirectoryIsOpenedAsOne(t *testing.T) {
	disk := foldingDiskHolding("cfg/p.json")

	listing, err := openDirectory(disk, "cfg")

	if err != nil || listing == nil {
		t.Fatalf("openDirectory = %v, %v; want the directory", listing, err)
	}
	_ = listing.Close()
}
