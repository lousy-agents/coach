package gitrepo

import (
	"context"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("bounded git reads", func() {
	It("rejects oversized git stdout without buffering unbounded output", func() {
		originalGit := commandContext
		DeferCleanup(func() {
			commandContext = originalGit
		})

		commandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "python3", "-c", "print('x'*100)")
		}

		_, err := RunBytesBounded(".", 10, 64<<10, time.Second, "show", "HEAD:project.json")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("stdout exceeded"))
	})

	It("drains git stdout and stderr concurrently so a large stderr write cannot deadlock", func() {
		originalGit := commandContext
		DeferCleanup(func() {
			commandContext = originalGit
		})

		commandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "python3", "-c", `
import sys
sys.stderr.write("e" * (1024 * 1024))
sys.stderr.flush()
sys.stdout.write("ok")
sys.stdout.flush()
`)
		}

		started := time.Now()
		data, err := RunBytesBounded(".", 1024, 2<<20, 2*time.Second, "show", "HEAD:project.json")
		elapsed := time.Since(started)

		Expect(err).NotTo(HaveOccurred(), "concurrent pipe drain must succeed without waiting for the wall-time budget; elapsed=%s", elapsed)
		Expect(elapsed).To(BeNumerically("<", 1500*time.Millisecond), "sequential pipe reads hang until timeout; elapsed=%s", elapsed)
		Expect(string(data)).To(Equal("ok"))
	})
})
