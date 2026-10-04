package codesignalcli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func body_tsAnalyzerAssetAcceptanceTest_producesAPrivateTempDirectoryWhoseContentsAreByt_66() {
	dir, cleanup, err := MaterializeTSAnalyzer(context.Background())
	Expect(err).NotTo(HaveOccurred())
	defer cleanup()

	var got []string
	Expect(fs.WalkDir(tsAnalyzerAssetFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
		Expect(walkErr).NotTo(HaveOccurred())
		if d.IsDir() {
			return nil
		}
		got = append(got, p)

		want, readErr := fs.ReadFile(tsAnalyzerAssetFS, p)
		Expect(readErr).NotTo(HaveOccurred())

		have, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		Expect(readErr).NotTo(HaveOccurred(), "materialized file missing: %s", p)
		Expect(have).To(Equal(want), "content mismatch for %s", p)
		return nil
	})).To(Succeed())
	Expect(got).NotTo(BeEmpty())

	shimInfo, err := os.Stat(filepath.Join(dir, "coach-ts-project-sidecar"))
	Expect(err).NotTo(HaveOccurred())
	Expect(shimInfo.Mode()&0o111).NotTo(BeZero(), "materialized sidecar shim must be executable (embed.FS itself never reports the source's executable bit)")

	pkg, err := os.ReadFile(filepath.Join(dir, "package.json"))
	Expect(err).NotTo(HaveOccurred(), "materialized analyzer root must carry package.json so Node does not walk above it for a package scope")
	Expect(string(pkg)).To(Equal("{\"type\":\"module\"}\n"))
}

func body_tsAnalyzerAssetAcceptanceTest_127(name string, boom error) error {
	if name == "project-sidecar/resolve.js" {
		return boom
	}
	return nil
}
