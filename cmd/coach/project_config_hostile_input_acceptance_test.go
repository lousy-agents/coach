package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// expectRefusedWith runs a baseline scan with the given --project-config value and requires the section 8.4
// refusal: exit 2, no report, the stable token and the reason fragment on standard error.
func expectRefusedWith(repo, path, token, reason string) string {
	stdout, stderrBytes, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", path)
	stderr := string(stderrBytes)
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
	ExpectWithOffset(1, stdout).To(BeEmpty(), "no report on a refused policy path")
	ExpectWithOffset(1, stderr).To(ContainSubstring(token))
	ExpectWithOffset(1, stderr).To(ContainSubstring(reason))
	return stderr
}

var _ = Describe("coach codesignal: --project-config path shapes are refused before the disk or git is asked", func() {
	DescribeTable("a value that is not a normalized repository-relative path",
		func(path, token, reason string) {
			repo := commitExplicitPolicyRepo()
			expectRefusedWith(repo, path, token, reason)
		},
		Entry("a detour through a parent that comes back", "policies/../policies/p.json", "project_config_invalid", "path must be normalized and remain inside the repository"),
		Entry("a leading ./", "./policies/p.json", "project_config_invalid", "path must be normalized and remain inside the repository"),
		Entry("a doubled slash", "policies//p.json", "project_config_invalid", "path must be normalized and remain inside the repository"),
		Entry("a trailing slash", "policies/p.json/", "project_config_invalid", "path must be normalized and remain inside the repository"),
		Entry("the repository root itself", ".", "project_config_invalid", "path must be normalized and remain inside the repository"),
		Entry("backslash separators", `policies\p.json`, "project_config_invalid", "path must use repository-relative slash separators"),
	)

	DescribeTable("a value that is a well-formed name for something that is not there",
		func(path string) {
			repo := commitExplicitPolicyRepo()
			expectRefusedWith(repo, path, "project_config_missing", "was not found on disk at the repository root")
		},
		Entry("one space", " "),
		Entry("a tab", "\t"),
		Entry("leading spaces before a real policy's name", "  policies/p.json"),
		Entry("a value that looks like a flag", "--base"),
		Entry("a path 2500 components long", strings.Repeat("a/", 2500)+"x.json"),
		Entry("a path 200 directories deep", strings.Repeat("d/", 200)+"x.json"),
		Entry("a tilde and an unexpanded variable", "~/$HOME/x.json"),
	)

	It("refuses a file name longer than the file system allows as unreachable on disk, not as missing", func() {
		repo := commitExplicitPolicyRepo()

		expectRefusedWith(repo, strings.Repeat("n", 300)+".json", "project_config_invalid", "cannot be checked on disk: file name too long")
	})

	DescribeTable("a path inside .git, under which git tracks nothing, whether or not it exists on disk",
		func(path string) {
			repo := commitExplicitPolicyRepo()
			expectRefusedWith(repo, path, "project_config_invalid", "must not be inside the .git directory")
		},
		Entry("a file in .git", ".git/config"),
		Entry("another file in .git", ".git/HEAD"),
		Entry("the .git directory", ".git"),
		Entry("a path inside .git that does not exist", ".git/nope.json"),
	)

	DescribeTable("a directory that is committed and on disk",
		func(path, reason string) {
			repo := commitExplicitPolicyRepo()
			expectRefusedWith(repo, path, "project_config_invalid", reason)
		},
		Entry("a committed directory", ".lousy-agents", "committed as a directory"),
	)
})

var _ = Describe("coach codesignal: a committed policy under a hostile name is scanned and nothing in the name runs", func() {
	DescribeTable("the name is only ever an argument",
		func(name string) {
			repo := commitScannableGoRepo()
			commitFile(repo, name, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", name)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
			_, statErr := os.Stat(filepath.Join(repo, "PWN"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "a shell metacharacter in the name ran: %s", name)
		},
		Entry("a leading dash", "-p.json"),
		Entry("a leading dash and an equals sign", "--x=1.json"),
		Entry("a double quote", `q"uote.json`),
		Entry("backticks", "`touch PWN`.json"),
		Entry("semicolons", "a;touch PWN;b.json"),
		Entry("command substitution", "$(touch PWN).json"),
		Entry("a BEL control byte", "bell\x07.json"),
		Entry("an ESC control sequence", "esc\x1b[31mred.json"),
		Entry("an apostrophe", "it's.json"),
	)

	It("scans a policy 120 directories deep", func() {
		repo := commitScannableGoRepo()
		deep := strings.TrimSuffix(strings.Repeat("d/", 120), "/") + "/coach.json"
		commitFile(repo, deep, singleRootPolicyJSON)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", deep)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())
	})
})

var _ = Describe("coach codesignal: an explicit --project-config entry that cannot be the policy is refused whatever it points at", func() {
	const canary = "TOP-SECRET-CANARY-7f3a91"

	outsideFile := func() string {
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "outside.json")
		Expect(os.WriteFile(path, []byte(`{"canary":"`+canary+`","schema_version":"1","roots":["."]}`), 0o644)).To(Succeed())
		return path
	}

	It("refuses a symbolic link at the path that points outside the repository, and prints nothing from the target", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.Symlink(outsideFile(), full)).To(Succeed())

		stderr := expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "is a symbolic link, not a regular file")

		Expect(stderr).NotTo(ContainSubstring(canary))
	})

	It("refuses a parent directory that is a symbolic link to a directory outside the repository, and prints nothing from the target", func() {
		repo := commitScannableGoRepo()
		outside := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(outside, "p.json"), []byte(`{"canary":"`+canary+`","schema_version":"1","roots":["."]}`), 0o644)).To(Succeed())
		Expect(os.Symlink(outside, filepath.Join(repo, "linked"))).To(Succeed())

		stderr := expectRefusedWith(repo, "linked/p.json", "project_config_invalid", "is reached through a symbolic link")

		Expect(stderr).NotTo(ContainSubstring(canary))
	})

	It("refuses a symbolic link at the path that points at itself", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.Symlink("p.json", full)).To(Succeed())

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "is a symbolic link, not a regular file")
	})

	It("refuses a symbolic link at the path that points at a directory", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.Symlink(filepath.Join(repo, "policies"), full)).To(Succeed())

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "is a symbolic link, not a regular file")
	})

	It("names the linked component when a symbolic link sits two directories above the policy", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "a/b/p.json", singleRootPolicyJSON)
		Expect(os.Rename(filepath.Join(repo, "a", "b"), filepath.Join(repo, "a", "real"))).To(Succeed())
		Expect(os.Symlink("real", filepath.Join(repo, "a", "b"))).To(Succeed())

		stderr := expectRefusedWith(repo, "a/b/p.json", "project_config_invalid", "is reached through a symbolic link")

		Expect(stderr).To(ContainSubstring(`"a/b"`))
	})
})

var _ = Describe("coach codesignal: an explicit --project-config entry of another kind is refused", func() {
	It("refuses a directory at the path when nothing is committed there", func() {
		repo := commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, explicitPolicyPath), 0o755)).To(Succeed())

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "is not a regular file")
	})

	It("refuses a directory at the path when a directory of that name is what is committed", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, explicitPolicyPath+"/inner.json", singleRootPolicyJSON)

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "committed as a directory")
	})

	It("refuses a path whose parent directory cannot be searched", func() {
		if os.Geteuid() == 0 {
			Skip("root can search any directory")
		}
		repo := commitExplicitPolicyRepo()
		parent := filepath.Join(repo, "policies")
		Expect(os.Chmod(parent, 0o600)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(parent, 0o755) })

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "cannot be checked on disk: permission denied")
	})
})

const (
	hostileScanBound = 10 * time.Second
	invalidJSONWord  = `(?i)json`
	oversizeWord     = `(?i)budget|too large|exceed`
)

// runScanWithin fails the spec if the scan has not finished within bound, which is how a read that waits on or
// drains what the path holds shows itself.
func runScanWithin(repo string, bound time.Duration, args ...string) (stdout, stderr []byte, exitCode int) {
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	command := exec.CommandContext(ctx, commandPath, append([]string{"codesignal", "--baseline", "--format=json"}, args...)...)
	command.Dir = repo
	var outBuf, errBuf bytes.Buffer
	command.Stdout, command.Stderr = &outBuf, &errBuf
	err := command.Run()
	ExpectWithOffset(1, ctx.Err()).NotTo(HaveOccurred(), "the scan did not finish within %s; stderr: %s", bound, errBuf.String())
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return outBuf.Bytes(), errBuf.Bytes(), 0
}

func expectHostilePolicyOnDiskRefusedWithin(repo, reason string) {
	stdout, stderr, exitCode := runScanWithin(repo, hostileScanBound, "--project-config", explicitPolicyPath)

	ExpectWithOffset(1, exitCode).To(Equal(2), "the bytes on disk are judged, not the valid committed copy; stdout=%s stderr=%s", stdout, stderr)
	ExpectWithOffset(1, stdout).To(BeEmpty(), "no report on a refused policy")
	ExpectWithOffset(1, string(stderr)).To(ContainSubstring("project_config_invalid"))
	ExpectWithOffset(1, string(stderr)).To(ContainSubstring(explicitPolicyPath))
	ExpectWithOffset(1, string(stderr)).To(MatchRegexp(reason), "the reason")
}

var _ = Describe("coach codesignal: the bytes on disk are judged, whatever a valid committed policy says (#168 A2b)", func() {
	DescribeTable("hostile bytes at an explicit --project-config path are refused with a reason, promptly",
		func(onDisk func() []byte, reason string) {
			repo := commitExplicitPolicyRepo()
			Expect(os.WriteFile(filepath.Join(repo, explicitPolicyPath), onDisk(), 0o644)).To(Succeed())

			expectHostilePolicyOnDiskRefusedWithin(repo, reason)
		},
		Entry("an empty file", func() []byte { return nil }, invalidJSONWord),
		Entry("random-looking binary", func() []byte { return []byte("\x00\xff\xfe\x80binary\x00\x01") }, invalidJSONWord),
		Entry("a byte-order mark before the JSON", func() []byte { return append([]byte("\xef\xbb\xbf"), singleRootPolicyJSON...) }, invalidJSONWord),
		Entry("200000 levels of nesting", func() []byte { return []byte(strings.Repeat("[", 200000) + strings.Repeat("]", 200000)) }, invalidJSONWord),
		Entry("a 5 MiB file", func() []byte { return []byte(strings.Repeat(" ", 5<<20) + singleRootPolicyJSON) }, oversizeWord),
	)

	It("refuses a sparse 300 MiB file at the path without reading it through", func() {
		repo := commitExplicitPolicyRepo()
		file, err := os.OpenFile(filepath.Join(repo, explicitPolicyPath), os.O_WRONLY|os.O_TRUNC, 0o644)
		Expect(err).NotTo(HaveOccurred())
		Expect(file.Truncate(300 << 20)).To(Succeed())
		Expect(file.Close()).To(Succeed())

		expectHostilePolicyOnDiskRefusedWithin(repo, oversizeWord)
	})
})
