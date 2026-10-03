package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func body_projectTsSetupPreviewAcceptanceTest_truthfullyDisclosesArgvOnDiskEffectNetworkScript_67(kind, wantExecutable string, wantArgs []string, wantLockfileBasename string, wantSuppressionSubstrings []string) {
	preview, err := codesignalcli.BuildSetupPreview(
		codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
		projectPackageManager(kind),
		"/tmp/example-root",
	)
	Expect(err).NotTo(HaveOccurred())

	Expect(preview.Executable).To(Equal(wantExecutable))
	Expect(preview.Args).To(Equal(wantArgs))

	Expect(preview.ExpectedChanges).To(ContainSubstring("node_modules"), "every frozen row's actual on-disk mutation is node_modules, not the lockfile")
	Expect(preview.ExpectedChanges).NotTo(
		MatchRegexp(setupPreviewLockfileRewriteClaimPattern),
		"none of the frozen rows can rewrite a lockfile -- npm ci and pnpm/bun's --frozen-lockfile install all fail instead of writing one",
	)
	if wantLockfileBasename != "" {
		Expect(preview.ExpectedChanges).To(ContainSubstring(wantLockfileBasename), "must name the lockfile basename this argv reads and never writes")
	} else {
		// Bun recognizes two lockfile variants (bun.lock, bun.lockb) and
		// BuildSetupPreview is not told which this repository has --
		// the disclosure must not cite either specific basename.
		Expect(preview.ExpectedChanges).NotTo(Or(ContainSubstring("bun.lock"), ContainSubstring("bun.lockb")), "must not name a specific lockfile variant it cannot confirm exists")
	}

	Expect(preview.NetworkDisclosure).To(And(ContainSubstring("network"), ContainSubstring("registry")), "must truthfully disclose that this command may reach the package registry")
	for _, wantSuppression := range wantSuppressionSubstrings {
		Expect(preview.ScriptSuppressionPolicy).To(ContainSubstring(wantSuppression), "must truthfully disclose every flag this row's argv actually passes to suppress scripts/config hazards -- a shared, one-size-fits-all disclosure string would silently under-disclose a row like pnpm's that carries an extra flag")
	}
	Expect(preview.Timeout).To(Equal(codesignalcli.SetupPreviewTimeout), "must disclose the bounded timeout that will actually be enforced")
}
