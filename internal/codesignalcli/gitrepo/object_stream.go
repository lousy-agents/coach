package gitrepo

import (
	"bufio"
	"fmt"
	"io"
)

// objectStream is the responses of one `git cat-file --batch`, and what is done with each object.
type objectStream struct {
	reader     *bufio.Reader
	limit      int64
	visit      func(oid string, content []byte) error
	visitLarge largeObjectVisit
}

// next reads the response for oid and hands the object on. The response it returns says tooLarge only for an
// object that was read past: one that visitLarge took has been written.
func (s *objectStream) next(oid string) (objectResponse, error) {
	response, err := readObjectResponse(s.reader, oid, s.limit)
	if err != nil || !response.found {
		return response, err
	}
	if response.tooLarge {
		response.tooLarge, err = s.pastTheLimit(oid, response.size)
		return response, err
	}
	return response, s.visit(oid, response.content)
}

// pastTheLimit offers an object past the limit on one object to visitLarge, then reads past what is left of it
// and the newline after it. It reports whether the object was left.
func (s *objectStream) pastTheLimit(oid string, size int64) (left bool, err error) {
	body := &io.LimitedReader{R: s.reader, N: size}
	taken := false
	if s.visitLarge != nil {
		if taken, err = s.visitLarge(oid, size, body); err != nil {
			return false, err
		}
	}
	if err := readPast(s.reader, body); err != nil {
		return false, fmt.Errorf("reading past the cat-file content for %s: %w", oid, err)
	}
	return !taken, nil
}

// readPast reads what is left of an object in body, and the newline after it. A stream that is cut inside the
// object ends before the newline, and the read of it fails.
func readPast(reader *bufio.Reader, body io.Reader) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	_, err := reader.Discard(1)
	return err
}
