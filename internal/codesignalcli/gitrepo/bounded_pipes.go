package gitrepo

import (
	"context"
	"io"
	"time"
)

// childPipeGrace is how long the readers get, after the context that bounds a read has ended, to see the
// end of the stream on their own. Killing git closes its end at once, so a stream that is still open after
// this has another holder: a child of git.
const childPipeGrace = time.Second

// closePipesOfAGitThatOutlivesItsContext closes the read ends of the pipes when the readers are still
// waiting childPipeGrace after the context ended, so that a child of git holding the write ends cannot keep
// a bounded read waiting for as long as the child lives. A reader that is closed under returns an error,
// which finishGitBoundedRead ranks below the bound that ended the read.
func closePipesOfAGitThatOutlivesItsContext(ctx context.Context, readersDone <-chan struct{}, grace time.Duration, pipes ...io.Closer) {
	select {
	case <-readersDone:
		return
	case <-ctx.Done():
	}
	select {
	case <-readersDone:
	case <-time.After(grace):
		for _, pipe := range pipes {
			_ = pipe.Close()
		}
	}
}
