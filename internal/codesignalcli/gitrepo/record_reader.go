package gitrepo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// maxListingRecordBytes bounds one record of a streamed listing. A record is a mode, a type, an object id and a
// path, and a path is not longer than the 4096 bytes git and the file systems it runs on allow, so a record past
// this is not a record: it is output that git did not produce, and reading on would hold it all.
const maxListingRecordBytes = 64 << 10

func visitRecords(reader *bufio.Reader, visit func(record []byte) error) error {
	var record []byte
	for {
		var err error
		record, err = readRecord(reader, record[:0])
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err == nil {
			err = visit(record)
		}
		if err != nil {
			return err
		}
	}
}

// readRecord reads up to the next NUL into buffer and returns the record without it, io.EOF at the clean end
// of the stream, and io.ErrUnexpectedEOF when the stream ends inside a record. A record of more than
// maxListingRecordBytes is refused whatever the size of the reader's own buffer: the bound is on the record, and
// is checked both when the buffer fills and when the NUL is found.
func readRecord(reader *bufio.Reader, buffer []byte) ([]byte, error) {
	for {
		chunk, err := reader.ReadSlice(0)
		buffer = append(buffer, chunk...)
		switch {
		case err == nil:
			return withinTheBound(buffer[:len(buffer)-1])
		case errors.Is(err, bufio.ErrBufferFull):
			if _, tooLong := withinTheBound(buffer); tooLong != nil {
				return nil, tooLong
			}
		case errors.Is(err, io.EOF) && len(buffer) == 0:
			return nil, io.EOF
		case errors.Is(err, io.EOF):
			return nil, io.ErrUnexpectedEOF
		default:
			return nil, err
		}
	}
}

func withinTheBound(record []byte) ([]byte, error) {
	if len(record) > maxListingRecordBytes {
		return nil, fmt.Errorf("a record of the listing is longer than %d bytes", maxListingRecordBytes)
	}
	return record, nil
}
