package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// objectOutcome is the objects that visit did not get: the ones the repository does not have, and the ones
// past the limit on one object, which were read past and not held.
type objectOutcome struct {
	missing  []string
	tooLarge []string
	// unreadable are the objects git said were missing and named in an error: they are in the object store and
	// damaged. said is what git printed about them.
	unreadable []string
	said       string
}

// largeObjectVisit is offered an object past the limit on one object, as a stream of size bytes in body, in
// place of its content. It reports whether it took the object: one that read its bytes, however it used them.
// An object it does not take is read past without being held, and is reported in the outcome. Whatever it has
// not read of an object it took is read past as well.
type largeObjectVisit func(oid string, size int64, body io.Reader) (taken bool, err error)

// readObjects asks one `git cat-file --batch` for each object in oids, in order, and hands the content of
// every object that is there to visit. An object past the limit on one object is not handed over and not held:
// it is reported in the outcome, and the read goes on with the next. cat-file returns the stored bytes and
// applies no filter, and with lazy fetching off (gitCommand) it says "missing" for an object a partial clone
// has not got instead of fetching it.
func readObjects(dir string, oids []string, budget snapshotBudget, visit func(oid string, content []byte) error) (outcome objectOutcome, err error) {
	return readObjectsStreaming(dir, oids, budget, visit, nil)
}

// readObjectsStreaming is readObjects with a way to take an object past the limit on one object: visitLarge
// is offered it as a stream, a chunk of it at a time, so that it is never held.
func readObjectsStreaming(dir string, oids []string, budget snapshotBudget, visit func(oid string, content []byte) error, visitLarge largeObjectVisit) (outcome objectOutcome, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), budget.wall)
	defer cancel()
	cmd := commandContext(ctx, dir, "cat-file", "--batch")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// Killing git when the context ends does not end a child of git, and a child keeps the pipes open. exec
	// copies stderr into the buffer, which makes the delay shut the pipes one second after the context ends:
	// the read below returns, and Wait does not wait for the child. (With both streams read by hand, as
	// RunBytesBounded does, the delay shuts nothing, and closePipesOfAGitThatOutlivesItsContext does.)
	cmd.WaitDelay = childPipeGrace
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return objectOutcome{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return objectOutcome{}, err
	}
	if err := cmd.Start(); err != nil {
		return objectOutcome{}, fmt.Errorf("git cat-file --batch: %w", err)
	}
	go requestObjects(stdin, oids)

	stream := &objectStream{reader: bufio.NewReader(stdout), limit: budget.object, visit: visit, visitLarge: visitLarge}
	for _, oid := range oids {
		response, readErr := stream.next(oid)
		if readErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return objectOutcome{}, boundedBy(ctx, budget.wall, readErr)
		}
		outcome.record(oid, response)
	}
	if err := cmd.Wait(); err != nil {
		return objectOutcome{}, boundedBy(ctx, budget.wall, fmt.Errorf("git cat-file --batch: %w: %s", err, TerminalSafe(strings.TrimSpace(stderr.String()))))
	}
	outcome.separateUnreadable(stderr.String())
	return outcome, nil
}

func (o *objectOutcome) record(oid string, response objectResponse) {
	switch {
	case !response.found:
		o.missing = append(o.missing, oid)
	case response.tooLarge:
		o.tooLarge = append(o.tooLarge, oid)
	}
}

// boundedBy says that the wall time ended the read when it did, since what the read then sees is only a
// pipe that closed.
func boundedBy(ctx context.Context, wall time.Duration, err error) error {
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("reading the objects of the snapshot took longer than %s", wall)
	}
	return err
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
