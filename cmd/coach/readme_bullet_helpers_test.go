package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// readmeBullet is the README's list item that begins with start, as one line: its line breaks and indentation are
// single spaces. The item ends at the next list item, or at the blank line that ends the list.
func readmeBullet(start string) string {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	text := string(readme)
	begin := strings.Index(text, "\n- "+start)
	ExpectWithOffset(1, begin).To(BeNumerically(">=", 0), "the README no longer has the bullet that begins %q", start)
	item := text[begin+len("\n- "):]
	if end := strings.Index(item, "\n- "); end >= 0 {
		item = item[:end]
	}
	if end := strings.Index(item, "\n\n"); end >= 0 {
		item = item[:end]
	}
	return strings.Join(strings.Fields(item), " ")
}
