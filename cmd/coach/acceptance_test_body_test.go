package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_acceptanceTest_marksTheInsertedFindingChangedAndTheUntouchedFin_325() {
	repo := newTempGitRepo()
	base := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
	head := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc C(input *int) {\n\t*input = 3\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
	initialSHA := commitFile(repo, "a.go", base)
	commitFile(repo, "a.go", head)

	report, _ := runCoachCodesignal(repo, initialSHA)

	signals := signalsForPath(report, "a.go")
	Expect(signals).To(HaveLen(3))

	var aSignal, bSignal, cSignal *codesignal.Signal
	for i := range signals {
		switch signals[i].Subject {
		case "A:input":
			aSignal = &signals[i]
		case "B:input":
			bSignal = &signals[i]
		case "C:input":
			cSignal = &signals[i]
		}
	}
	Expect(aSignal).NotTo(BeNil())
	Expect(bSignal).NotTo(BeNil())
	Expect(cSignal).NotTo(BeNil())

	Expect(aSignal.Changed).To(BeFalse())
	Expect(bSignal.Changed).To(BeFalse())
	Expect(cSignal.Changed).To(BeTrue())
	Expect(cSignal.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
}

func body_acceptanceTest_classifiesTheAddedFileSSignalsAsIntroduced_438() {
	repo := newTempGitRepo()
	deleted := "package gone\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
	added := `package fresh

func tangle(n int) {
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							return
						}
					}
				}
			}
		}
	}
}
`
	initialSHA := commitFile(repo, "gone.go", deleted)

	rmCmd := exec.Command("git", "rm", "gone.go")
	rmCmd.Dir = repo
	output, err := rmCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git rm: %s", output)

	Expect(os.WriteFile(filepath.Join(repo, "fresh.go"), []byte(added), 0o644)).To(Succeed())
	addCmd := exec.Command("git", "add", "fresh.go")
	addCmd.Dir = repo
	output, err = addCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git add: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "replace gone.go with fresh.go")
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

	statusCmd := exec.Command("git", "diff", "--name-status", initialSHA, "HEAD")
	statusCmd.Dir = repo
	statusOut, err := statusCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Split(strings.TrimSpace(string(statusOut)), "\n")).To(ConsistOf("A\tfresh.go", "D\tgone.go"))

	report, _ := runCoachCodesignal(repo, initialSHA)

	Expect(hasDiagnostic(report, "unsupported_change_type", "fresh.go")).To(BeFalse())
	Expect(report.Summary.IntroducedSignals).To(BeNumerically(">", 0),
		"an added file must contribute introduced signals; got introduced=%d resolved=%d for %d signals",
		report.Summary.IntroducedSignals, report.Summary.ResolvedSignals, len(report.Signals))

	freshSignals := signalsForPath(report, "fresh.go")
	Expect(freshSignals).NotTo(BeEmpty())
	for i := range freshSignals {
		Expect(freshSignals[i].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	}

	goneSignals := signalsForPath(report, "gone.go")
	Expect(goneSignals).NotTo(BeEmpty())
	for i := range goneSignals {
		Expect(goneSignals[i].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
	}
}

func body_acceptanceTest_analyzesTheHEADPathWithoutEstablishingRenameCont_785() {
	repo := newTempGitRepo()
	initialSHA := commitFile(repo, "old.go", hiddenInputMutationGo)
	headSHA := renameFile(repo, "old.go", "new.go")
	expectCoachStatusPrefix(repo, initialSHA, "new.go", "R")

	report, _ := runCoachCodesignal(repo, initialSHA)

	Expect(hasDiagnostic(report, "unsupported_change_type", "new.go")).To(BeFalse(),
		"a rename's new path must be analyzed, not skipped as unsupported_change_type")
	signals := signalsForPath(report, "new.go")
	Expect(signals).NotTo(BeEmpty(), "HEAD content of a renamed file must yield signals")
	for i := range signals {
		Expect(signals[i].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")),
			"rename continuity is not established, so signals must not inherit introduced from added-file policy")
	}
	Expect(hasDiagnostic(report, "continuity_not_determined", "new.go")).To(BeTrue(),
		"the report must say continuity was not determined on the analyzed new path")
	continuity, found := diagnosticFor(report, "continuity_not_determined", "new.go")
	Expect(found).To(BeTrue())
	Expect(continuity.Side).To(Equal("head"),
		"rename/copy continuity detection is head-side only, so the diagnostic must identify head as the comparison side")
	Expect(continuity.Revision).To(Equal(headSHA),
		"the diagnostic's machine-readable revision must be the actual HEAD SHA the rename was analyzed at")
	Expect(continuity.Message).To(ContainSubstring("head revision "+headSHA),
		"the diagnostic's text must name the same side and revision as its machine-readable Side and Revision fields")
	Expect(signalsForPath(report, "old.go")).To(BeEmpty(), "the old path no longer exists at HEAD and must not appear in the report")

	textOut, textErr, textExit := runCoachCodesignalRaw(repo, initialSHA)
	Expect(textExit).To(Equal(0), "stderr: %s", textErr)
	Expect(string(textOut)).NotTo(ContainSubstring("not analyzed"),
		"an analyzed rename must not be described as unanalyzed")
	Expect(string(textOut)).To(ContainSubstring("head revision "+headSHA),
		"text output must name the same side and revision as the diagnostic's machine-readable Side and Revision fields")
	var document struct {
		Summary struct {
			FilesUnanalyzed *int `json:"files_unanalyzed"`
		} `json:"summary"`
	}
	jsonOut, jsonErr, jsonExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
	Expect(jsonExit).To(Equal(0), "stderr: %s", jsonErr)
	Expect(json.Unmarshal(jsonOut, &document)).To(Succeed())
	Expect(document.Summary.FilesUnanalyzed).To(BeNil(),
		"JSON must omit files_unanalyzed when every changed path was analyzed")
}
