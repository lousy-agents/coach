package gitrepo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func recordsOf(t *testing.T, stream io.Reader, bufferSize int) ([]int, error) {
	t.Helper()
	var lengths []int
	err := visitRecords(bufio.NewReaderSize(stream, bufferSize), func(record []byte) error {
		lengths = append(lengths, len(record))
		return nil
	})
	return lengths, err
}

// The bound is on the record: 64 KiB is read, one byte more is refused, and it does not depend on the size of the
// buffer the reader happens to have, which decides only when the check can run before the NUL is found.
func TestARecordIsRefusedPastExactly64KiBWhateverTheBufferOfTheReader(t *testing.T) {
	for _, bufferSize := range []int{64 << 10, 4096, 16} {
		assertTheBoundOfARecord(t, bufferSize, maxListingRecordBytes, true)
		assertTheBoundOfARecord(t, bufferSize, maxListingRecordBytes+1, false)
	}
	assertTheBoundOfARecord(t, 64<<10, 70000, false)
	assertTheBoundOfARecord(t, 64<<10, 131071, false)
	assertTheBoundOfARecord(t, 64<<10, 131072, false)
}

func assertTheBoundOfARecord(t *testing.T, bufferSize, length int, accepted bool) {
	t.Helper()
	t.Run(fmt.Sprintf("a record of %d bytes, read through a buffer of %d", length, bufferSize), func(t *testing.T) {
		lengths, err := recordsOf(t, strings.NewReader(strings.Repeat("a", length)+"\x00"), bufferSize)

		if accepted && (err != nil || len(lengths) != 1 || lengths[0] != length) {
			t.Errorf("records %v, error %v; want the one record of %d bytes read", lengths, err, length)
		}
		if !accepted && (err == nil || !strings.Contains(err.Error(), "longer than 65536 bytes")) {
			t.Errorf("records %v, error %v; want a record of %d bytes refused as longer than 65536 bytes", lengths, err, length)
		}
	})
}

// A record of exactly the bound that the stream ends in is cut short, and not refused for its size.
func TestARecordOfExactlyTheBoundThatEndsTheStreamIsCutShort(t *testing.T) {
	_, err := recordsOf(t, strings.NewReader(strings.Repeat("a", maxListingRecordBytes)), 64<<10)

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("error %v; want the stream cut inside a record", err)
	}
}

// A read that fails with something other than the end of the stream is that error, and no record is visited for
// it: a persistent failure that was swallowed would be read as one empty record after another.
func TestAReadThatFailsInsideARecordIsThatError(t *testing.T) {
	failure := errors.New("the pipe broke")
	stream := io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(failure))

	err := visitRecords(bufio.NewReader(stream), func([]byte) error { return errors.New("visited a record that is not one") })

	if !errors.Is(err, failure) {
		t.Errorf("error %v; want the failed read", err)
	}
}
