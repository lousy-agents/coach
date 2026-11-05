package main

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestPathValueFromEnvironReadsOnlyTheVariableNamedPATH(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"alone", "PATH=/runtime\n", "/runtime"},
		{"among other variables", "HOME=/h\nPATH=/runtime\nPWD=/w\n", "/runtime"},
		{"after a MANPATH line", "MANPATH=/usr/share/man\nPATH=/runtime\n", "/runtime"},
		{"after a NODE_PATH line", "NODE_PATH=/leak\nPATH=/runtime\n", "/runtime"},
		{"only a NODE_PATH line", "NODE_PATH=/leak\n", ""},
		{"absent", "HOME=/h\n", ""},
		{"value that contains PATH=", "PATH=/a/PATH=/b\n", "/a/PATH=/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			NewWithT(t).Expect(pathValueFromEnviron(c.env)).To(Equal(c.want))
		})
	}
}
