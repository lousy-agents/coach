package gitrepo

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// The listing of a large repository is past any limit on one read, and it is read all the same: what is held is
// a record. This one is 70 MB, past the 64 MiB a git read is held to.
func TestAListingPastTheBudgetOfOneReadIsReadWhole(t *testing.T) {
	withListingScript(t, "yes aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | head -n 700000 | tr '\\n' '\\000'")
	var bytes int

	err := streamListing("", time.Minute, []string{"ls-tree"}, func(record []byte) error {
		bytes += len(record) + 1
		return nil
	})

	if err != nil || bytes != 700000*(len("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")+1) {
		t.Errorf("streamListing = %v after %d bytes; want all 700000 records read, without an error about a size", err, bytes)
	}
	if bytes <= 64<<20 {
		t.Errorf("the listing is %d bytes, which is not past the 64 MiB a git read is held to", bytes)
	}
}

func TestAListingThatEndsInsideARecordIsAnError(t *testing.T) {
	withTreeListing(t, "whole\x00cut sho")

	records, err := listedRecords()

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("streamListing = %v after %q; want the stream cut inside a record refused", err, records)
	}
}

func TestARecordPastTheLimitIsRefusedAndGitIsEnded(t *testing.T) {
	withListingScript(t, "yes aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | tr -d '\\n'")
	started := time.Now()

	err := streamListing("", time.Minute, []string{"ls-tree"}, func([]byte) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "longer than 65536 bytes") {
		t.Errorf("streamListing = %v; want the record that never ends refused", err)
	}
	if time.Since(started) > 15*time.Second {
		t.Errorf("streamListing took %s to refuse a record: git was left running", time.Since(started))
	}
}
