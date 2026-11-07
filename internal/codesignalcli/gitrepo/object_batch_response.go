package gitrepo

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// objectResponse is what git answered for one object: its content, or that it is missing, or that it is
// past the limit and was discarded unread.
type objectResponse struct {
	content  []byte
	found    bool
	tooLarge bool
}

// readObjectResponse reads "<oid> <type> <size>", the content and a newline, or "<oid> missing". An object
// past limit is not allocated: its bytes and the newline are read past, so the next response starts where
// the next request's answer does.
func readObjectResponse(reader *bufio.Reader, oid string, limit int64) (objectResponse, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return objectResponse{}, fmt.Errorf("reading the cat-file response for %s: %w", oid, err)
	}
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[1] == "missing" {
		return objectResponse{}, nil
	}
	if len(fields) != 3 {
		return objectResponse{}, fmt.Errorf("malformed cat-file header for %s: %q", oid, header)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return objectResponse{}, fmt.Errorf("malformed cat-file size for %s: %q", oid, header)
	}
	if size > limit {
		if _, err := io.CopyN(io.Discard, reader, size+1); err != nil {
			return objectResponse{}, fmt.Errorf("reading past the cat-file content for %s: %w", oid, err)
		}
		return objectResponse{found: true, tooLarge: true}, nil
	}
	content := make([]byte, size)
	if _, err := io.ReadFull(reader, content); err != nil {
		return objectResponse{}, fmt.Errorf("reading the cat-file content for %s: %w", oid, err)
	}
	if _, err := reader.Discard(1); err != nil {
		return objectResponse{}, fmt.Errorf("reading the end of the cat-file content for %s: %w", oid, err)
	}
	return objectResponse{content: content, found: true}, nil
}
