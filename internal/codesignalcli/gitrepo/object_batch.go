package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// readObjects asks one `git cat-file --batch` for each object in oids, in order, and hands the content of
// every object that is there to visit. It returns the objects that are not in the repository. cat-file
// returns the stored bytes and applies no filter, and with lazy fetching off (gitCommand) it says
// "missing" for an object a partial clone has not got instead of fetching it.
func readObjects(dir string, oids []string, visit func(oid string, content []byte) error) (missing []string, err error) {
	cmd := gitCommand(context.Background(), dir, "cat-file", "--batch")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	go requestObjects(stdin, oids)

	reader := bufio.NewReader(stdout)
	for _, oid := range oids {
		content, found, readErr := readObjectResponse(reader, oid)
		if readErr == nil && found {
			readErr = visit(oid, content)
		}
		if readErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, readErr
		}
		if !found {
			missing = append(missing, oid)
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return missing, nil
}

// requestObjects writes the requests while the responses are read, so that a long list cannot fill both
// pipes and stop both processes.
func requestObjects(stdin io.WriteCloser, oids []string) {
	writer := bufio.NewWriter(stdin)
	for _, oid := range oids {
		_, _ = writer.WriteString(oid + "\n")
	}
	_ = writer.Flush()
	_ = stdin.Close()
}
