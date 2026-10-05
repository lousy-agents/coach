package main

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript (package-manager adapter detection)", func() {
	When("the manager binary is probed for its version", func() {
		It("runs it in a private directory with only PATH and HOME, so a committed config in the repository cannot steer what it reports", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			managerDir := writeRecordingStubPackageManagerScript("npm", "11.4.1")
			path := managerDir + string(os.PathListSeparator) + pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"), "the probe must have run at all, or the assertions below prove nothing")

			cwds := readStubPackageManagerCwds(managerDir)
			Expect(cwds).NotTo(BeEmpty())
			for _, cwd := range cwds {
				Expect(cwd).NotTo(Equal(repo), "the probe must not run inside the repository under analysis")
			}

			env := readStubPackageManagerEnv(managerDir)
			Expect(env).NotTo(BeEmpty())
			for _, name := range env {
				Expect(name).To(BeElementOf("PATH", "HOME", "PWD", "SHLVL", "_"),
					"only PATH and HOME may be forwarded; %q came from the parent environment (the rest are set by the shell running the stub itself)", name)
			}
		})
	})
})
