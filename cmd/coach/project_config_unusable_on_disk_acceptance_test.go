package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const explicitPolicyPath = "policies/p.json"

// commitExplicitPolicyRepo commits a valid explicit policy plus valid
// discovered policies, so a read from HEAD or a fallback to a discovered policy
// would produce a report that these specs could see.
func commitExplicitPolicyRepo() string {
	repo := commitScannableGoRepo()
	commitFile(repo, explicitPolicyPath, singleRootPolicyJSON)
	commitFile(repo, coachConfigPath, singleRootPolicyJSON)
	commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
	return repo
}

func expectExplicitPolicyRefused(repo, path, token string) (stderr string) {
	stdout, stderrBytes, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", path)
	ExpectWithOffset(1, exitCode).To(Equal(2), "the committed copy must never stand in for the file on disk; stdout=%s stderr=%s", stdout, stderrBytes)
	ExpectWithOffset(1, stdout).To(BeEmpty(), "no report")
	ExpectWithOffset(1, string(stderrBytes)).To(ContainSubstring(path))
	ExpectWithOffset(1, string(stderrBytes)).To(ContainSubstring(token))
	return string(stderrBytes)
}

var _ = Describe("coach codesignal: an explicit --project-config whose entry at the path is unusable on disk never reads the committed copy", func() {
	It("refuses a dangling symbolic link, naming the link", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.Symlink("/nonexistent/x.json", full)).To(Succeed())

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("symbolic link"))
	})

	It("refuses a live symbolic link to a valid policy too", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "policies", "real.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		Expect(os.Symlink("real.json", full)).To(Succeed())

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("symbolic link"))
	})

	It("refuses a path reached through a symbolic link to a directory, naming the link", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "real/p.json", singleRootPolicyJSON)
		Expect(os.Symlink("real", filepath.Join(repo, "linked"))).To(Succeed())

		stderr := expectExplicitPolicyRefused(repo, "linked/p.json", "project_config_invalid")

		Expect(stderr).To(ContainSubstring("symbolic link"))
		Expect(stderr).NotTo(ContainSubstring("not committed"), "committing cannot fix a path that runs through a link")
	})

	It("refuses a path through a link that points at itself as reached through a symbolic link, not as missing", func() {
		repo := commitScannableGoRepo()
		Expect(os.Symlink("loop", filepath.Join(repo, "loop"))).To(Succeed())

		stderr := expectExplicitPolicyRefused(repo, "loop/p.json", "project_config_invalid")

		Expect(stderr).To(ContainSubstring("reached through a symbolic link"))
		Expect(stderr).NotTo(ContainSubstring("project_config_missing"))
	})

	It("reports a file standing in place of a parent directory as missing", func() {
		repo := commitExplicitPolicyRepo()
		Expect(os.RemoveAll(filepath.Join(repo, "policies"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "policies"), []byte("x\n"), 0o644)).To(Succeed())

		expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_missing")
	})

	It("refuses a directory in place of the file, naming that it is not a regular file", func() {
		repo := commitExplicitPolicyRepo()
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.Mkdir(full, 0o755)).To(Succeed())

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("not a regular file"))
	})

	It("fails closed when the path cannot be checked, naming the error", func() {
		if os.Geteuid() == 0 {
			Skip("root can search a mode 000 directory, so this state cannot be built")
		}
		repo := commitExplicitPolicyRepo()
		locked := filepath.Join(repo, "policies")
		Expect(os.Chmod(locked, 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(locked, 0o755) })

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("cannot be checked on disk"))
	})

	It("prints only the cause when the path cannot be checked, never the file-system error's own text", func() {
		if os.Geteuid() == 0 {
			Skip("root can search a mode 000 directory, so this state cannot be built")
		}
		repo := commitScannableGoRepo()
		locked := filepath.Join(repo, "ev\x1b[31mRED")
		Expect(os.Mkdir(locked, 0o755)).To(Succeed())
		Expect(os.Chmod(locked, 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(locked, 0o755) })

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--project-config", "ev\x1b[31mRED/p.json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
		Expect(string(stderr)).To(ContainSubstring("permission denied"))
		Expect(string(stderr)).NotTo(ContainSubstring("\x1b"), "the escape character reached the terminal:\n%q", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("statat"), "the file-system call's own text is not echoed:\n%s", stderr)
	})

	It("reports a never-committed path under a regular file as missing, not invalid", func() {
		repo := commitScannableGoRepo()

		expectExplicitPolicyRefused(repo, "a.go/p.json", "project_config_missing")
	})
})

var _ = Describe("coach codesignal: an explicit --project-config the disk decides is unusable is not fixed by a new policy or a restore", func() {
	unusableEntries := []struct {
		name         string
		needsNonRoot bool
		replaceFile  func(full string)
	}{
		{"a dangling symbolic link", false, func(full string) { Expect(os.Symlink("/nonexistent/x.json", full)).To(Succeed()) }},
		{"a directory", false, func(full string) { Expect(os.Mkdir(full, 0o755)).To(Succeed()) }},
		{"a named pipe", false, func(full string) { Expect(syscall.Mkfifo(full, 0o644)).To(Succeed()) }},
		{"a regular file that nobody can read", true, func(full string) {
			Expect(os.WriteFile(full, []byte(singleRootPolicyJSON), 0o000)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(full, 0o644) })
		}},
	}

	for _, entry := range unusableEntries {
		prepare := func() string {
			if entry.needsNonRoot && os.Geteuid() == 0 {
				Skip("root can read a mode 000 file, so this state cannot be built")
			}
			repo := commitExplicitPolicyRepo()
			full := filepath.Join(repo, explicitPolicyPath)
			Expect(os.Remove(full)).To(Succeed())
			entry.replaceFile(full)
			return repo
		}

		It("suggests no new policy in the scan when the path holds "+entry.name, func() {
			repo := prepare()

			stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

			Expect(stderr).NotTo(ContainSubstring(suggestCommand), "a new policy cannot replace what the disk holds at the path:\n%s", stderr)
		})

		It("names neither author_policy nor restore_policy in --check-project when the path holds "+entry.name, func() {
			repo := prepare()

			doc := expectCheckProject(repo, "--project-config", explicitPolicyPath)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	}
})

var _ = Describe("coach codesignal: an explicit --project-config that exists but cannot be read is refused on disk, committed or not", func() {
	unreadablePolicy := func(repo string) {
		if os.Geteuid() == 0 {
			Skip("root can read a mode 000 file, so this state cannot be built")
		}
		full := filepath.Join(repo, explicitPolicyPath)
		Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
		Expect(os.WriteFile(full, []byte(singleRootPolicyJSON), 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(full, 0o644) })
	}

	It("names permission denied and suggests no new policy when the file was never committed", func() {
		repo := commitScannableGoRepo()
		unreadablePolicy(repo)

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("permission denied"))
		Expect(stderr).NotTo(ContainSubstring(suggestCommand), "a new policy cannot replace a file that is there:\n%s", stderr)
	})

	It("names permission denied and does not read the committed copy when the file is committed", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, explicitPolicyPath, singleRootPolicyJSON)
		Expect(os.Chmod(filepath.Join(repo, explicitPolicyPath), 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(filepath.Join(repo, explicitPolicyPath), 0o644) })
		if os.Geteuid() == 0 {
			Skip("root can read a mode 000 file, so this state cannot be built")
		}

		stderr := expectExplicitPolicyRefused(repo, explicitPolicyPath, "project_config_invalid")

		Expect(stderr).To(ContainSubstring("permission denied"))
	})

	It("names neither author_policy nor restore_policy in --check-project when the file was never committed", func() {
		repo := commitScannableGoRepo()
		unreadablePolicy(repo)

		doc := expectCheckProject(repo, "--project-config", explicitPolicyPath)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
	})
})
