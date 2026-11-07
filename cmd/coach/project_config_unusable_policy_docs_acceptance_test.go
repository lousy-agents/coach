package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A TypeScript scan opens no guided authoring for a policy that exists and cannot be used, with or without a terminal,
// because the only session it has ends by asking the user to save a candidate over the file. It says in words how to get past
// the policy, in the sentence --prepare-compiler prints (unusablePolicyStep). The documents promised the session; these specs
// hold them to the decision, and fail on the sentences that promised it.
var _ = Describe("the documents: a TypeScript policy that exists and cannot be used gets no session and the words for getting past it", func() {
	const stepInWords = "fix the policy, or move it aside and commit the move, then run guided policy authoring"

	It("says so in the CLI contract's --project-config row, and keeps the offer for a layerless valid file", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("An invalid canonical file is an error and does not fall back. A TypeScript scan opens no guided authoring for it, with or without a terminal, because the only session it has ends by asking the user to save a candidate over the file; it exits `2` with no report and says in words to " + stepInWords))
		Expect(text).To(ContainSubstring("On a terminal, a TypeScript scan offers guided policy authoring after the report when the file is valid but declares no layers and the scan exits `0`."))
		Expect(text).NotTo(ContainSubstring("a TypeScript scan offers guided policy authoring for it instead of the scan"))
	})

	It("says so in the CLI contract's --check-project row", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("`--check-project` prints the action by its kind alone, `author_policy`, with no words of its own, and it is the scan and the `--prepare-compiler` refusal that say to " + stepInWords))
		Expect(text).To(ContainSubstring("a TypeScript scan opens no session for a policy that exists, with or without a terminal"))
		Expect(text).NotTo(ContainSubstring("a scan on a terminal opens guided authoring and prints a replacement by itself"))
		Expect(text).NotTo(ContainSubstring("the text of the action is to fix the policy"), "--check-project prints no such text")
		Expect(text).NotTo(ContainSubstring("say that in the same words"))
	})

	It("holds that row to what --check-project prints: the action by its kind, with no words of its own", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, coachConfigPath, invalidPolicyText)

		doc := expectCheckProject(repo)

		authoring := nextActionsNamed(doc, "author_policy")
		Expect(authoring).To(HaveLen(1), "next actions: %+v", doc.NextActions)
		Expect(authoring[0].Detail).To(BeEmpty(), "the row of docs/cli-codesignal.md says --check-project prints the action by its kind alone; the words for getting past the policy are the scan's and --prepare-compiler's, so a detail here makes the row false")
	})

	It("says so in the config convention", func() {
		text := documentText("docs/conventions/lousy-agents-config.md")

		Expect(text).To(ContainSubstring("A TypeScript scan that finds an invalid canonical document opens no guided policy authoring, with or without a terminal"))
		Expect(text).To(ContainSubstring("and says in words to " + stepInWords + " to write a new one"))
		Expect(text).NotTo(ContainSubstring("a TypeScript scan that finds an invalid canonical document offers guided policy authoring instead of only reporting the error"))
	})

	It("requires it in the PRD, and drops the exception that offered a session for an invalid file", func() {
		prd := documentText("docs/product/prd.md")

		Expect(prd).To(ContainSubstring("WHEN a TypeScript scan omits `--project-config` and a `.lousy-agents/coach.json` on disk fails schema validation, with or without a controlling terminal, THE Coach CLI shall open no guided policy authoring"))
		Expect(prd).To(ContainSubstring("print the one sentence `coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one.`, which `--prepare-compiler` also prints for the same policy"))
		Expect(prd).To(ContainSubstring("For a TypeScript scan it shall also say in words to " + stepInWords))
		Expect(prd).NotTo(ContainSubstring("Except where section 8.5 offers guided policy authoring, THE Coach CLI shall exit with status `2`"))
		Expect(prd).NotTo(ContainSubstring("THE Coach CLI shall offer guided policy authoring instead of the scan"))
	})

	It("describes the capability table row without the offer for an invalid file", func() {
		prd := documentText("docs/product/prd.md")

		Expect(prd).To(ContainSubstring("A discovered file that is schema-invalid gets no session, with or without a terminal: the scan says in words to fix it, or move it aside and commit the move, then run guided policy authoring."))
		Expect(prd).NotTo(ContainSubstring("An invalid file gets the offer instead of the scan"))
	})
})
