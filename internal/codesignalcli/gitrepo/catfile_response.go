package gitrepo

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (r *RevisionFileReader) request(path string) ([]byte, error) {
	if _, err := io.WriteString(r.stdin, r.revisionSHA+":"+path+"\x00"); err != nil {
		return nil, fmt.Errorf("writing cat-file request for %q: %w", path, err)
	}

	header, err := r.stdout.ReadString('\x00')
	if err != nil {
		return nil, fmt.Errorf("reading cat-file response header for %q: %w", path, err)
	}
	header = strings.TrimSuffix(header, "\x00")

	fields := strings.Fields(header)
	if len(fields) == 2 && fields[1] == "missing" {
		return nil, fmt.Errorf("%q is missing at %s", path, r.revisionSHA)
	}
	if len(fields) != 3 {
		return nil, fmt.Errorf("malformed cat-file header for %q: %q", path, header)
	}

	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return nil, fmt.Errorf("malformed cat-file size for %q: %q", path, header)
	}
	if size > r.budget.object {
		return nil, r.discard(path, size)
	}
	return r.content(path, size)
}

// discard reads past an object that is over the limit without holding it, and its trailing NUL, so the next
// response starts where the next request's answer does.
func (r *RevisionFileReader) discard(path string, size int64) error {
	if _, err := io.CopyN(io.Discard, r.stdout, size+1); err != nil {
		return fmt.Errorf("discarding cat-file content for %q: %w", path, err)
	}
	return &ObjectTooLargeError{Path: path, Size: size, Limit: r.budget.object}
}

func (r *RevisionFileReader) content(path string, size int64) ([]byte, error) {
	content := make([]byte, size)
	if _, err := io.ReadFull(r.stdout, content); err != nil {
		return nil, fmt.Errorf("reading cat-file content for %q: %w", path, err)
	}

	// Each response ends with exactly one trailing NUL after the object's
	// content (per -Z), regardless of the object's own bytes.
	if _, err := r.stdout.Discard(1); err != nil {
		return nil, fmt.Errorf("reading cat-file trailing NUL for %q: %w", path, err)
	}

	return content, nil
}
