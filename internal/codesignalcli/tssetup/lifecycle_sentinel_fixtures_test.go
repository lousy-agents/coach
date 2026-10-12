package tssetup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func copyFileTree(src, dst string) {
	Expect(filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		return copyFileTreeEntry(p, d, err, src, dst)
	})).To(Succeed())
}

func copyFileTreeEntry(p string, d fs.DirEntry, err error, src string, dst string) error {
	if err != nil {
		return err
	}
	rel, relErr := filepath.Rel(src, p)
	if relErr != nil {
		return relErr
	}
	target := filepath.Join(dst, rel)
	if d.IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	content, readErr := os.ReadFile(p)
	if readErr != nil {
		return readErr
	}
	return os.WriteFile(target, content, 0o644)
}

func startLifecycleSentinelRegistry() *httptest.Server {
	tarball := lifecycleSentinelTarball()
	sha1sum := sha1.Sum(tarball)
	sha512sum := sha512.Sum512(tarball)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())
	baseURL := "http://" + ln.Addr().String()
	tarballPath := "/" + lifecycleSentinelPackageName + "/-/" + lifecycleSentinelPackageName + "-" + lifecycleSentinelVersion + ".tgz"
	tarballURL := baseURL + tarballPath
	versionDoc := map[string]any{
		"name":    lifecycleSentinelPackageName,
		"version": lifecycleSentinelVersion,
		"dist": map[string]any{
			"tarball":   tarballURL,
			"shasum":    hex.EncodeToString(sha1sum[:]),
			"integrity": "sha512-" + base64.StdEncoding.EncodeToString(sha512sum[:]),
		},
	}
	packument, err := json.Marshal(map[string]any{
		"name": lifecycleSentinelPackageName,
		"dist-tags": map[string]string{
			"latest": lifecycleSentinelVersion,
		},
		"versions": map[string]any{
			lifecycleSentinelVersion: versionDoc,
		},
	})
	Expect(err).NotTo(HaveOccurred())
	versionJSON, err := json.Marshal(versionDoc)
	Expect(err).NotTo(HaveOccurred())

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveLifecycleSentinelRegistry(w, r, tarball, tarballPath, packument, versionJSON)
	}))
	srv.Listener = ln
	srv.Start()
	DeferCleanup(srv.Close)
	return srv
}

func serveLifecycleSentinelRegistry(w http.ResponseWriter, r *http.Request, tarball []byte, tarballPath string, packument []byte, versionJSON []byte) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/" + lifecycleSentinelPackageName:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(packument)
	case "/" + lifecycleSentinelPackageName + "/" + lifecycleSentinelVersion:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(versionJSON)
	case tarballPath:
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(tarball)
	default:
		GinkgoWriter.Printf("lifecycle-sentinel registry: unexpected %s %s\n", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

// lifecycleSentinelFixtureDir is the checked-in local npm package
// (cmd/coach/testdata/mise/lifecycle-sentinel/lifecycle-sentinel@1.2.3)
// whose pre/postinstall scripts write lifecycleSentinelFile if ever
// executed. Its directory name embeds "@1.2.3" -- verified empirically
// against mise 2026.9.5: mise's npm:file: backend (both the default aube
// backend and the npm.shell_out=true real-npm backend) re-derives its
// internal "specifier" from the directory name, so a toolSpec of
// "npm:file:<this-dir>" only resolves correctly when the directory's own
// name already carries the exact version suffix mise expects.
func lifecycleSentinelFixtureDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "cmd", "coach", "testdata", "mise", "lifecycle-sentinel", "lifecycle-sentinel@1.2.3")
}

// copyLifecycleSentinelFixture copies the checked-in lifecycle-sentinel
// fixture into a fresh directory under destParent, preserving its
// "@1.2.3"-suffixed leaf directory name (lifecycleSentinelFixtureDir's own
// doc comment explains why that suffix must survive the copy). Installing
// straight from the checked-in fixture would let a real suppression
// regression write lifecycleSentinelFile into this repository's own
// worktree instead of failing the spec, rather than into a disposable copy.
func copyLifecycleSentinelFixture(destParent string) (fixtureDir string) {
	fixtureDir = filepath.Join(destParent, filepath.Base(lifecycleSentinelFixtureDir()))
	copyFileTree(lifecycleSentinelFixtureDir(), fixtureDir)
	return fixtureDir
}

// assertLifecycleSentinelFixtureIsLive proves the lifecycle-sentinel
// fixture's pre/postinstall scripts are genuinely capable of firing, by
// installing a disposable copy of it with a real `npm install -g` that
// carries neither mise's own npm-backend --ignore-scripts flag nor Coach's
// own suppression env var (npm_config_ignore_scripts is explicitly
// stripped from the ambient environment first, in case the host happens to
// set it). Without this, a negative "the sentinel never appears" assertion
// elsewhere in the same spec would pass just as easily if the fixture were
// inert -- AC-15's false-green-resistant sentinel requirement.
func assertLifecycleSentinelFixtureIsLive(destParent string) {
	liveCopy := copyLifecycleSentinelFixture(destParent)
	prefix := filepath.Join(destParent, "npm-global-prefix")
	Expect(os.MkdirAll(prefix, 0o755)).To(Succeed())

	cmd := exec.Command("npm", "install", "-g", "file:"+liveCopy, "--prefix", prefix)
	cmd.Env = filteredEnviron(miseNpmScriptSuppressionEnvKey)
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "positive control npm install: %s", output)
	Expect(anyFileNamed(liveCopy, lifecycleSentinelFile)).To(BeTrue(), "expected the fixture's pre/postinstall script to fire when nothing suppresses it, or the negative assertion this spec makes elsewhere would be vacuous")
}

func lifecycleSentinelTarball() []byte {
	pkgJSON, err := os.ReadFile(filepath.Join(lifecycleSentinelFixtureDir(), "package.json"))
	Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "package/package.json",
		Mode:     0o644,
		Size:     int64(len(pkgJSON)),
		Typeflag: tar.TypeReg,
	}
	Expect(tw.WriteHeader(hdr)).To(Succeed())
	_, err = tw.Write(pkgJSON)
	Expect(err).NotTo(HaveOccurred())
	Expect(tw.Close()).To(Succeed())
	Expect(gz.Close()).To(Succeed())
	return buf.Bytes()
}
