package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// cliContractBullet is the CLI contract's list item that begins with start, as one line: its line breaks and
// indentation are single spaces. The item ends at the next list item, or at the blank line that ends the list.
func cliContractBullet(start string) string {
	contract, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli-codesignal.md"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	text := string(contract)
	begin := strings.Index(text, "\n- "+start)
	ExpectWithOffset(1, begin).To(BeNumerically(">=", 0), "the CLI contract no longer has the bullet that begins %q", start)
	item := text[begin+len("\n- "):]
	if end := strings.Index(item, "\n- "); end >= 0 {
		item = item[:end]
	}
	if end := strings.Index(item, "\n\n"); end >= 0 {
		item = item[:end]
	}
	return strings.Join(strings.Fields(item), " ")
}
