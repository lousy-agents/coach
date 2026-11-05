package main

import (
	"os"
	"strings"
)

func answeredYes(stdin *os.File) bool {
	answer := strings.ToLower(strings.TrimSpace(readLineWithoutReadAhead(stdin)))
	return answer == "y" || answer == "yes"
}

// readLineWithoutReadAhead takes one byte at a time because a buffered
// reader would swallow answers typed ahead for the next prompt, which reads
// stdin again.
func readLineWithoutReadAhead(stdin *os.File) string {
	var line []byte
	next := make([]byte, 1)
	for {
		n, err := stdin.Read(next)
		switch {
		case n == 1 && next[0] == '\n':
			return string(line)
		case n == 1:
			line = append(line, next[0])
		}
		if err != nil {
			return string(line)
		}
	}
}
