package main

// commitStubToolchainTypeScriptRepo is a TypeScript repository whose toolchain the specs stub, with a policy
// committed at policyPath.
func commitStubToolchainTypeScriptRepo(policyPath, policy string) string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	commitFile(repo, policyPath, policy)
	return repo
}
