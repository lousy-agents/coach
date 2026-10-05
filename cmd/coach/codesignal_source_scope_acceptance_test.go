package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("a Go build target has changed production, test-only, unreachable, and build-tag-excluded files", func() {
		It("excludes test-only findings from default production scope without requiring a build target", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {}\n")

			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "default production scope should complete without a build target; stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "shipping/shipping.go")).To(HaveLen(1), "a non-test Go file must remain visible when production membership is unknown")
			Expect(signalsForPath(&report, "shipping/shipping_test.go")).To(BeEmpty(), "default production scope must not report a test-only Go finding when no build target is supplied")
		})

		It("defaults to production scope and reports only findings that ship in the selected build target", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "go.mod", "module example.com/scopefixture\n\ngo 1.25.0\n")
			commitFile(repo, "cmd/app/main.go", "package main\n\nimport _ \"example.com/scopefixture/shipping\"\n\nfunc main() {}\n")
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {}\n")
			commitFile(repo, "internal/tool/tool.go", "package tool\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/disabled.go", "//go:build never\n\npackage shipping\n\nfunc Update(input *int) {}\n")

			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "internal/tool/tool.go", "package tool\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/disabled.go", "//go:build never\n\npackage shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--build-target", "./cmd/app", "--format=json")
			Expect(exitCode).To(Equal(0), "production scope should analyze the selected build target; stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "shipping/shipping.go")).To(HaveLen(1), "the production dependency must remain visible")
			Expect(signalsForPath(&report, "shipping/shipping_test.go")).To(BeEmpty(), "test-only findings must not appear in the default production scope")
			Expect(signalsForPath(&report, "internal/tool/tool.go")).To(BeEmpty(), "a package outside the selected build target must not be claimed to ship")
			Expect(signalsForPath(&report, "shipping/disabled.go")).To(BeEmpty(), "a file excluded by its Go build constraint must not be claimed to ship")
			Expect(sourceScopeForPath(stdout, "shipping/shipping.go")).To(Equal("production"), "JSON must identify why a visible finding is in the production report")
		})

		It("includes the complete changed source set when the user explicitly requests all scope", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "go.mod", "module example.com/scopeallfixture\n\ngo 1.25.0\n")
			commitFile(repo, "cmd/app/main.go", "package main\n\nimport _ \"example.com/scopeallfixture/shipping\"\n\nfunc main() {}\n")
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {}\n")

			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--scope=all", "--format=json")
			Expect(exitCode).To(Equal(0), "all scope should retain exploratory findings; stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "shipping/shipping.go")).To(HaveLen(1))
			Expect(signalsForPath(&report, "shipping/shipping_test.go")).To(HaveLen(1), "all scope must include test-only findings for exploratory use")
			Expect(sourceScopeForPath(stdout, "shipping/shipping_test.go")).To(Equal("test_only"), "all scope must identify that the extra finding is test-only")
		})
	})

	When("a TypeScript project defines its production compilation in tsconfig.json", func() {
		It("uses that production configuration by default and excludes changed test-only findings", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "tsconfig.json", "{\"include\":[\"src/**/*.ts\"],\"exclude\":[\"test/**/*.ts\"]}\n")
			commitFile(repo, "src/app.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "test/app.test.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")

			commitFile(repo, "src/app.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")
			commitFile(repo, "test/app.test.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(signalsForPath(report, "src/app.ts")).To(HaveLen(1), "a file included by tsconfig.json must remain visible in production scope")
			Expect(signalsForPath(report, "test/app.test.ts")).To(BeEmpty(), "a file excluded by tsconfig.json must not appear in default production scope")
		})

		It("uses tsconfig.json membership for TSX files as well as TypeScript files", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "tsconfig.json", "{\"include\":[\"src/**/*.tsx\"],\"exclude\":[\"test/**/*.tsx\"]}\n")
			commitFile(repo, "src/panel.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "test/panel.test.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")

			commitFile(repo, "src/panel.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")
			commitFile(repo, "test/panel.test.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(signalsForPath(report, "src/panel.tsx")).To(HaveLen(1), "a TSX file included by tsconfig.json must remain visible in production scope")
			Expect(signalsForPath(report, "test/panel.test.tsx")).To(BeEmpty(), "a TSX test file excluded by tsconfig.json must not appear in default production scope")
		})
	})

	When("production membership cannot be established for a changed supported file", func() {
		It("fails open by retaining the finding and labelling it unknown in JSON", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "orphan.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "orphan.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "a file with unknown scope must remain an advisory result; stderr: %s", stderr)
			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "orphan.ts")).To(HaveLen(1), "unknown scope must not silently hide a supported finding")
			Expect(sourceScopeForPath(stdout, "orphan.ts")).To(Equal("unknown"), "JSON must make the uncertain production membership explicit to automation")
		})
	})
})
