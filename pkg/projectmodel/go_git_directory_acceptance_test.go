package projectmodel_test

import (
	"context"
	"io/fs"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// A snapshot is written to a directory that go then loads. A .git directory in it would be a repository that git
// and go's version control stamping take for one, and its config names programs git runs. git refuses to check
// out a tree that holds one, and so does every place Coach writes a committed tree to disk.
var _ = Describe("the Go builders refuse a snapshot that holds a git directory", func() {
	snapshot := func(gitName string) fs.FS {
		return fstest.MapFS{
			"go.mod":            {Data: []byte("module example.com/planted\n\ngo 1.22\n")},
			"main.go":           {Data: []byte("package main\n\nfunc main() {}\n")},
			gitName + "/HEAD":   {Data: []byte("ref: refs/heads/main\n")},
			gitName + "/config": {Data: []byte("[core]\n\tfsmonitor = \"touch /tmp/ran; false\"\n")},
		}
	}

	DescribeTable("before it writes anything",
		func(build func(fs.FS) error) {
			err := build(snapshot(".git"))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("git directory"))
			Expect(err.Error()).To(ContainSubstring(`the tree holds the path ".git"`), "the error names the path: %v", err)
		},
		Entry("call graph", func(snapshot fs.FS) error {
			_, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			return err
		}),
		Entry("layer bypass", func(snapshot fs.FS) error {
			_, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{})
			return err
		}),
		Entry("reachability", func(snapshot fs.FS) error {
			_, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			return err
		}),
	)

	It("refuses the spellings a case-folding or NTFS file system reads as the same directory", func() {
		for _, name := range []string{".GIT", "git~1", ".git."} {
			_, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot(name), projectmodel.CallGraphOptions{})

			Expect(err).To(HaveOccurred(), "a tree with %q was loaded", name)
		}
	})
})
