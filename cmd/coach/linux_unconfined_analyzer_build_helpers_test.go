package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func buildUnconfinedAnalyzerCoach() string {
	root := repositoryRoot()
	tmp, err := os.MkdirTemp("", "coach-unconfined-analyzer-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, tmp)

	vfsSrc := filepath.Join(root, "internal", "codesignalcli", "tsanalyzerasset", "project-sidecar", "vfs.js")
	analyzeSrc := filepath.Join(root, "internal", "codesignalcli", "tsanalyzerasset", "project-sidecar", "analyze.js")
	runtimeSrc := filepath.Join(root, "internal", "codesignalcli", "ts_runtime.go")

	vfsDst := filepath.Join(tmp, "vfs.js")
	analyzeDst := filepath.Join(tmp, "analyze.js")
	runtimeDst := filepath.Join(tmp, "ts_runtime.go")

	vfs, err := os.ReadFile(vfsSrc)
	Expect(err).NotTo(HaveOccurred())
	listingBlock := `        getAccessibleEntries: (directoryName) => {
            const result = base.getAccessibleEntries ? base.getAccessibleEntries(directoryName) : undefined;
            return result === undefined ? { files: [], directories: [] } : result;
        },`
	hostListing := `        getAccessibleEntries: (directoryName) => {
            const result = base.getAccessibleEntries ? base.getAccessibleEntries(directoryName) : undefined;
            if (result !== undefined) return result;
            try {
                const names = readdirSync(directoryName, { withFileTypes: true });
                return {
                    files: names.filter((d) => d.isFile() || d.isSymbolicLink()).map((d) => d.name),
                    directories: names.filter((d) => d.isDirectory()).map((d) => d.name),
                };
            } catch {
                return { files: [], directories: [] };
            }
        },`
	patchedVFS := "import { readdirSync } from \"node:fs\";\n" + strings.Replace(string(vfs), listingBlock, hostListing, 1)
	Expect(patchedVFS).NotTo(Equal(string(vfs)), "unconfined analyzer must restore host filesystem listing fall-through")
	Expect(strings.Contains(patchedVFS, listingBlock)).To(BeFalse(), "confined empty-listing wrapper must be gone")
	Expect(os.WriteFile(vfsDst, []byte(patchedVFS), 0o644)).To(Succeed())

	analyze, err := os.ReadFile(analyzeSrc)
	Expect(err).NotTo(HaveOccurred())
	patchedAnalyze := strings.Replace(string(analyze), "return new ApiCtor({ fs: snapshot.fs, tsserverPath });", "return new ApiCtor({ fs: snapshot.fs });", 1)
	Expect(patchedAnalyze).NotTo(Equal(string(analyze)), "unconfined analyzer must omit tsserverPath")
	snapshotLine := "    const snapshot = buildProjectSnapshot(opts.files, opts.compiler.createVirtualFileSystem);\n"
	typeRootsWalk := snapshotLine + `    for (const f of opts.files) {
        if (!f.path.endsWith("tsconfig.json")) continue;
        try {
            const cfg = JSON.parse(Buffer.from(f.content_b64, "base64").toString("utf8"));
            for (const root of cfg.compilerOptions?.typeRoots ?? []) {
                snapshot.fs.getAccessibleEntries?.(root);
            }
        } catch { }
    }
`
	Expect(strings.Contains(patchedAnalyze, snapshotLine)).To(BeTrue())
	patchedAnalyze = strings.Replace(patchedAnalyze, snapshotLine, typeRootsWalk, 1)
	Expect(os.WriteFile(analyzeDst, []byte(patchedAnalyze), 0o644)).To(Succeed())

	runtimeBytes, err := os.ReadFile(runtimeSrc)
	Expect(err).NotTo(HaveOccurred())
	patchedRuntime := strings.Replace(string(runtimeBytes),
		`"--native-package=" + compiler.NativePackagePath,`,
		`"--native-package=" + compiler.NativePackagePath,`+"\n\t\t\""+unconfinedArgvHook+`",`,
		1)
	Expect(patchedRuntime).NotTo(Equal(string(runtimeBytes)), "throwaway must spawn through the go test-only argv hook")
	Expect(os.WriteFile(runtimeDst, []byte(patchedRuntime), 0o644)).To(Succeed())

	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{
		vfsSrc:     vfsDst,
		analyzeSrc: analyzeDst,
		runtimeSrc: runtimeDst,
	}}
	overlayPath := filepath.Join(tmp, "overlay.json")
	payload, err := json.Marshal(overlay)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(overlayPath, payload, 0o644)).To(Succeed())

	bin := filepath.Join(tmp, "coach")
	build := exec.Command("go", "build", "-a", "-overlay", overlayPath, "-o", bin, ".")
	build.Dir = filepath.Join(root, "cmd", "coach")
	out, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building throwaway unconfined analyzer: %s", out)
	Expect(bin).NotTo(HavePrefix(filepath.Join(root, "dist")), "throwaway must not be a GoReleaser dist path")
	Expect(bin).NotTo(ContainSubstring("tsanalyzerasset"), "throwaway must not be the checked-in embed")
	return bin
}
