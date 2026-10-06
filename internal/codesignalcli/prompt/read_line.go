// Package prompt reads answers from an interactive session one line at a time.
package prompt

import (
	"bufio"
)

// ReadLine sets unreadable on any read error, not only io.EOF: a closed
// stdin or detached tty will never produce a different answer later, and
// treating only io.EOF as exhausted would spin promptRetryOrCancel forever.
func ReadLine(reader *bufio.Reader) (line string, unreadable bool) {
	line, err := reader.ReadString('\n')
	return line, err != nil
}
