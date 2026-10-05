package main

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

func body_projectTsSetupExecuteAcceptanceTest_confinesTheChildToExactlyPATHAndHOMEDroppingEver_199() {
	workDir := newSetupExecutionWorkDir()
	stubDir := writeStubSetupExecutable("npm")
	GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

	// Each of these would, if forwarded, re-enable or redirect the
	// very thing --ignore-scripts and the frozen argv are supposed to
	// guarantee: a re-enabled lifecycle-script setting, an injected
	// Node startup flag, and a registry override.
	GinkgoT().Setenv("npm_config_ignore_scripts", "false")
	GinkgoT().Setenv("NODE_OPTIONS", "--require ./evil.js")
	GinkgoT().Setenv("npm_config_registry", "http://127.0.0.1:9/attacker")

	preview, err := tssetup.BuildPreview(tssetup.Choice{Kind: tssetup.ChoiceProjectPackage}, projectPackageManager("npm"), workDir)
	Expect(err).NotTo(HaveOccurred())

	result, execErr := tssetup.Execute(context.Background(), preview, true)
	Expect(execErr).NotTo(HaveOccurred())
	Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

	observedKeys := readStubSetupEnvKeys(stubDir, "npm")
	Expect(observedKeys).NotTo(ContainElement("npm_config_ignore_scripts"), "an ambient lifecycle-script override must never reach the child")
	Expect(observedKeys).NotTo(ContainElement("NODE_OPTIONS"), "an ambient Node startup-flag injection must never reach the child")
	Expect(observedKeys).NotTo(ContainElement("npm_config_registry"), "an ambient registry override must never reach the child")
	Expect(observedKeys).To(ContainElement("PATH"), "the child needs PATH to resolve npm and node")
	for _, key := range observedKeys {
		Expect(key).To(BeElementOf("PATH", "HOME", "PWD", "SHLVL", "_"),
			"only PATH and HOME may be forwarded; %q came from the parent environment (the rest are set by the shell running the stub itself)", key)
	}
}
