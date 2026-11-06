package main

import (
	"os"
	"path/filepath"
	"strings"

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
		Entry("a path inside .git that does not exist", ".git/nope.json"),
		Entry("a tilde and an unexpanded variable", "~/$HOME/x.json"),
	)

	It("refuses a file name longer than the file system allows as unreachable on disk, not as missing", func() {
		repo := commitExplicitPolicyRepo()

		expectRefusedWith(repo, strings.Repeat("n", 300)+".json", "project_config_invalid", "cannot be checked on disk: file name too long")
	})

	DescribeTable("a path inside .git, which exists on disk but is never committed",
		func(path, reason string) {
			repo := commitExplicitPolicyRepo()
			expectRefusedWith(repo, path, "project_config_invalid", reason)
		},
		Entry("a file in .git", ".git/config", "is not committed at revision"),
		Entry("another file in .git", ".git/HEAD", "is not committed at revision"),
		Entry("the .git directory", ".git", "is not a regular file"),
		Entry("a committed directory", ".lousy-agents", "is not a regular file"),
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

		expectRefusedWith(repo, explicitPolicyPath, "project_config_invalid", "is not a regular file")
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

	DescribeTable("the bytes on disk are never what is read: a committed policy still decides the scan",
		func(onDisk func() []byte) {
			repo := commitExplicitPolicyRepo()
			Expect(os.WriteFile(filepath.Join(repo, explicitPolicyPath), onDisk(), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", explicitPolicyPath)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
		},
		Entry("an empty file", func() []byte { return nil }),
		Entry("random-looking binary", func() []byte { return []byte("\x00\xff\xfe\x80binary\x00\x01") }),
		Entry("a byte-order mark before the JSON", func() []byte { return append([]byte("\xef\xbb\xbf"), singleRootPolicyJSON...) }),
		Entry("200000 levels of nesting", func() []byte { return []byte(strings.Repeat("[", 200000) + strings.Repeat("]", 200000)) }),
		Entry("a 5 MiB file", func() []byte { return []byte(strings.Repeat(" ", 5<<20) + singleRootPolicyJSON) }),
	)
})
