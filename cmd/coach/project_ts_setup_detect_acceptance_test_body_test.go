package main

import (
	"encoding/json"
	"os"

	. "github.com/onsi/gomega"
)

func body_projectTsSetupDetectAcceptanceTest_reportsChecksPackageManagerFailPackageManagerVer_42() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

	path := pathWithStubNodeAndPackageManager("v24.9.9", "npm", "9.5.0")

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
	Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
	Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
	Expect(doc.Checks.PackageManager.FoundVersion).To(Equal("9.5.0"))
	Expect(gapCodes(doc)).To(ContainElement("package_manager_version_unsupported"))
	Expect(doc.Status).To(Equal("needs_prerequisite"))

	// pathWithStubNode deliberately excludes mise, so the mise_project
	// and mise_global choices are independently unverifiable here too
	// (SA-280-045) -- match on the npm adapter's own entry rather than
	// the last resolve_package_manager action.
	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "resolve_package_manager" && a.PackageManagerKind == "npm" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("resolve_package_manager"))
	Expect(action.Executable).To(BeFalse())
	Expect(action.PackageManagerKind).To(Equal("npm"))
	Expect(action.FoundVersion).To(Equal("9.5.0"))

	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	Expect(string(textStdout)).To(ContainSubstring("resolve_package_manager (executable=false) package_manager_kind=npm found_version=9.5.0"))
}

func body_projectTsSetupDetectAcceptanceTest_runsItInAPrivateDirectoryWithOnlyPATHAndHOMESoAC_121() {
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
}

func body_projectTsSetupDetectAcceptanceTest_withholdsYarnReportingChecksPackageManagerFailPa_214() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "yarn.lock", "# yarn lockfile v1\n")

	path := pathWithStubNode("v24.9.9")

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
	Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
	Expect(doc.Checks.PackageManager.Kind).To(Equal("yarn"))
	Expect(doc.Checks.PackageManager.Detail).NotTo(BeEmpty(), "Yarn's withholding must carry an informational note explaining why")

	Expect(nextActionKinds(doc)).To(ContainElement("resolve_package_manager"), "the executable=false assertion below must run against a next action that actually exists")
	for _, a := range doc.NextActions {
		if a.PackageManagerKind == "yarn" {
			Expect(a.Executable).To(BeFalse(), "Yarn must never be offered as an executable installation choice")
		}
	}
}

func body_projectTsSetupDetectAcceptanceTest_reportsChecksPackageManagerPassWithNoGapNamingTh_399() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

	path := pathWithStubNodeAndPackageManager("v24.9.9", "npm", "11.2.0")

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
	Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
	Expect(doc.Checks.PackageManager.Version).To(Equal("11.2.0"))
	// pathWithStubNode deliberately excludes mise, so mise_project and
	// mise_global independently surface their own unverifiable gap
	// (SA-280-045) -- assert only that no gap names the npm adapter
	// itself, since that is what this spec's fixture exercises.
	for _, gap := range doc.Gaps {
		Expect(gap.PackageManagerKind).NotTo(Equal("npm"))
	}
}
