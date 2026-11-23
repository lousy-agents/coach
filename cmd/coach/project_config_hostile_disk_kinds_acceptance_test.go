//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func makeAFifoAt(full string) {
	Expect(syscall.Mkfifo(full, 0o644)).To(Succeed())
}

func makeASocketAt(full string) {
	if err := syscall.Mknod(full, syscall.S_IFSOCK|0o644, 0); err != nil {
		Skip("this file system cannot hold a socket node: " + err.Error())
	}
}

var _ = Describe("coach codesignal: an entry that is neither a file nor a directory at the policy path is refused without waiting on it (#168 A2b)", func() {
	DescribeTable("the path holds",
		func(replaceWith func(full string), explicit bool) {
			repo := commitScannableGoRepo()
			policyPath, args := coachConfigPath, []string(nil)
			if explicit {
				repo = commitExplicitPolicyRepo()
				policyPath, args = explicitPolicyPath, []string{"--project-config", explicitPolicyPath}
			} else {
				Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			}
			full := filepath.Join(repo, policyPath)
			Expect(os.RemoveAll(full)).To(Succeed())
			replaceWith(full)

			stdout, stderr, exitCode := runScanWithin(repo, hostileScanBound, args...)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(policyPath))
			Expect(string(stderr)).To(Or(ContainSubstring("not_a_regular_file"), ContainSubstring("not a regular file")), "the reason")
		},
		Entry("a named pipe at the discovered path", makeAFifoAt, false),
		Entry("a socket at the discovered path", makeASocketAt, false),
		Entry("a named pipe at an explicit path", makeAFifoAt, true),
		Entry("a socket at an explicit path", makeASocketAt, true),
	)
})
