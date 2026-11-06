package gitrepo

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// readObjectResponse reads "<oid> <type> <size>", the content and a newline, or "<oid> missing".
func readObjectResponse(reader *bufio.Reader, oid string, limit int64) (content []byte, found bool, err error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, fmt.Errorf("reading the cat-file response for %s: %w", oid, err)
	}
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[1] == "missing" {
		return nil, false, nil
	}
	if len(fields) != 3 {
		return nil, false, fmt.Errorf("malformed cat-file header for %s: %q", oid, header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return nil, false, fmt.Errorf("malformed cat-file size for %s: %q", oid, header)
	}
	if int64(size) > limit {
		return nil, false, fmt.Errorf("object %s is %d bytes, past the %d-byte limit on one object of a snapshot", oid, size, limit)
	}
	content = make([]byte, size)
	if _, err := io.ReadFull(reader, content); err != nil {
		return nil, false, fmt.Errorf("reading the cat-file content for %s: %w", oid, err)
	}
	if _, err := reader.Discard(1); err != nil {
		return nil, false, fmt.Errorf("reading the end of the cat-file content for %s: %w", oid, err)
	}
	return content, true, nil
}
