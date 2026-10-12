package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise-tool resolution ignores a Bun declaration (AC-14, issue #354 out of scope)", func() {

	It("resolves the compiler from the npm:typescript entry alone, never surfacing a Bun version declared alongside it in the same [tools] table", func() {
		repo := newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "mise.toml", "[tools]\nbun = \"1.2.3\"\n\"npm:typescript\" = \"7.0.2\"\n")

		path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

		doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

		Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got %+v", doc.Checks.Compiler)
		Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the resolved version must come from the npm:typescript entry, never the bun entry's own version")
		Expect(gapCodes(doc)).To(BeEmpty())
		Expect(text).NotTo(ContainSubstring("1.2.3"), "a mise-declared Bun version must never be consumed as a runtime declaration anywhere in this flow's rendered report, got:\n%s", text)

		invocations := readStubMiseInvocations(miseDir)
		Expect(invocations).To(ContainElement("where npm:typescript@7.0.2"), "the npm:typescript entry must be located from, got invocations=%v", invocations)
		for _, invocation := range invocations {
			Expect(invocation).NotTo(ContainSubstring("1.2.3"), "the bun entry's own version must never appear in any mise invocation, got invocations=%v", invocations)
		}
	})
})
