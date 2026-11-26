package evalpatterns_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
)

const arrowSource = `package ledger

import "net/http"

type Server struct{ name string }

func NewServer(name string) *Server { return &Server{name: name} }

func NewLoaded() *Server {
	s := load()
	return s
}

func load() *Server { return nil }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func Handle(w http.ResponseWriter, r *http.Request) {}

func (s *Server) Name() string { return s.name }

func Arrow(xs [][]int, limit int) int {
	total := 0
	for _, x := range xs {
		if len(x) > 0 {
			for _, v := range x {
				if v > 0 && v <= limit {
					total += v
				}
			}
		}
	}
	return total
}
`

const pickSource = `package pick

func Pick(n int) string {
	switch {
	case n > 1:
		return "many"
	}
	if n == 1 {
		return "one"
	} else if n == 0 {
		return "none"
	}
	return "neg"
}
`

// standInTree is a small Go module whose shapes are known by hand:
// ledger/apply.go holds seven functions, among them Arrow (nesting 4,
// cognitive 11) in a file of four branch points; pick/pick.go holds Pick
// (nesting 1, cognitive 3) in a file of three. A test file, testdata and
// vendor hold code that would change every count if it were read.
func standInTree() string {
	GinkgoHelper()
	root := filepath.Join(GinkgoT().TempDir(), "standin")
	files := map[string]string{
		"go.mod":                        "module example.invalid/standin\n\ngo 1.26.0\n",
		"ledger/apply.go":               arrowSource,
		"pick/pick.go":                  pickSource,
		"ledger/apply_test.go":          "package ledger\n\nfunc NewTestOnly() int { if true { return 1 }; return 0 }\n",
		"ledger/testdata/broken.go":     "package broken\n\nfunc broken( {\n",
		"vendor/example.invalid/v/v.go": "package v\n\nfunc NewVendored() int { if true { return 1 }; return 0 }\n",
	}
	for name, content := range files {
		writeFile(filepath.Join(root, filepath.FromSlash(name)), content)
	}
	return root
}

// plantShims puts a go and a git on PATH that each create marker when run.
func plantShims(marker string) {
	GinkgoHelper()
	bin := GinkgoT().TempDir()
	for _, name := range []string{"go", "git"} {
		path := filepath.Join(bin, name)
		writeFile(path, "#!/bin/sh\ntouch '"+marker+"'\nexit 1\n")
		if err := os.Chmod(path, 0o755); err != nil {
			Fail(err.Error())
		}
	}
	GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
