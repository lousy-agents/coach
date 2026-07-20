package acceptanceharness_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// fakeRoundTripper is an offline http.RoundTripper stand-in that never
// dials any real network: it just returns a canned response, recording
// that it was invoked.
type fakeRoundTripper struct {
	called bool
	resp   *http.Response
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.called = true
	return f.resp, nil
}

var _ = Describe("ambient-credential guard", func() {
	Context("when known ambient-credential variables are present in the scanned environment", func() {
		It("reports each as a violation and Rejected() is true (AC: GitHub + AWS)", func() {
			environ := []string{
				"GITHUB_TOKEN=ghp_totallyfake",
				"AWS_ACCESS_KEY_ID=AKIAFAKEFAKEFAKE",
				"UNRELATED_VAR=hello",
			}

			result := acceptanceharness.ScanEnviron(environ)

			Expect(result.Rejected()).To(BeTrue())
			Expect(result.Found).To(ContainElement("GITHUB_TOKEN"))
			Expect(result.Found).To(ContainElement("AWS_ACCESS_KEY_ID"))
		})
	})

	Context("when none of the known ambient-credential variables are present", func() {
		It("reports no violations", func() {
			environ := []string{
				"UNRELATED_VAR=hello",
				"PATH=/usr/bin",
			}

			result := acceptanceharness.ScanEnviron(environ)

			Expect(result.Rejected()).To(BeFalse())
			Expect(result.Found).To(BeEmpty())
		})
	})

	Context("when a GitHub OAuth or model-provider credential variable is present in the scanned environment", func() {
		// These categories (GitHub OAuth client id/secret, and well-known
		// model-provider API keys) were reported missing from
		// AmbientCredentialVars in PR #80 review: a synthetic
		// OPENAI_API_KEY or GITHUB_CLIENT_SECRET passed ScanEnviron
		// undetected even though a real acceptance suite must not inherit
		// either ambiently.
		DescribeTable("is flagged as a violation",
			func(name, value string) {
				environ := []string{name + "=" + value, "UNRELATED_VAR=hello"}

				result := acceptanceharness.ScanEnviron(environ)

				Expect(result.Rejected()).To(BeTrue())
				Expect(result.Found).To(ContainElement(name))
			},
			Entry("GitHub OAuth client id", "GITHUB_CLIENT_ID", "fake-oauth-client-id"),
			Entry("GitHub OAuth client secret", "GITHUB_CLIENT_SECRET", "fake-oauth-client-secret"),
			Entry("OpenAI API key", "OPENAI_API_KEY", "sk-fake-openai-key"),
			Entry("Anthropic API key", "ANTHROPIC_API_KEY", "sk-ant-fake-key"),
			Entry("AWS shared config file override", "AWS_CONFIG_FILE", "/fake/home/.aws/config"),
		)

		It("is also flagged end-to-end by ScanProcessEnv against the real process environment", func() {
			// Isolate from whatever default credential file(s) may exist on
			// the real host running this suite, the same way the
			// file-check specs below do, so this assertion depends only on
			// the env vars this spec sets.
			tmpHome := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", tmpHome)
			GinkgoT().Setenv("GITHUB_CLIENT_SECRET", "fake-oauth-client-secret")
			GinkgoT().Setenv("OPENAI_API_KEY", "sk-fake-openai-key")

			result := acceptanceharness.ScanProcessEnv()

			Expect(result.Rejected()).To(BeTrue())
			Expect(result.Found).To(ContainElement("GITHUB_CLIENT_SECRET"))
			Expect(result.Found).To(ContainElement("OPENAI_API_KEY"))
		})
	})

	Context("when ScrubProcessEnv is called against the real process environment", func() {
		It("unsets a previously-set ambient-credential var so it is no longer present in os.Environ()", func() {
			// ScrubProcessEnv unsets every AmbientCredentialVars entry it
			// finds present in the *real* process environment, not just the
			// one this spec cares about. If the machine running this suite
			// already has some other entry (e.g. AWS_PROFILE) set in its
			// real ambient environment, calling ScrubProcessEnv here would
			// permanently unset it process-wide with no restoration. To
			// keep this spec isolated, set every single AmbientCredentialVars
			// entry via GinkgoT().Setenv first, so all of them become known,
			// test-owned values that Ginkgo will restore (to their original
			// value, or unset if originally absent) after this spec,
			// regardless of what ScrubProcessEnv scrubs.
			for _, name := range acceptanceharness.AmbientCredentialVars {
				GinkgoT().Setenv(name, "test-value-"+name)
			}

			scrubbed := acceptanceharness.ScrubProcessEnv()

			Expect(scrubbed).To(ConsistOf(acceptanceharness.AmbientCredentialVars))

			// ScanProcessEnv (called by ScrubProcessEnv, and again below)
			// also checks $HOME/.aws/credentials. Without repointing HOME
			// at a clean temp directory here, this assertion depends on
			// whether the real host running this suite happens to have that
			// file -- making mise run ci host-dependent (PR #80 review).
			// Isolate the same way the file-check specs below do.
			tmpHome := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", tmpHome)

			Expect(acceptanceharness.ScanProcessEnv().Rejected()).To(BeFalse())
		})
	})
})

var _ = Describe("default credential-file check", func() {
	Context("when the injected exists func reports an AmbientCredentialFiles path present", func() {
		It("includes that path in FoundFiles", func() {
			home := "/fake/home"
			present := filepath.Join(home, acceptanceharness.AmbientCredentialFiles[0])

			found := acceptanceharness.ScanCredentialFiles(home, func(path string) bool {
				return path == present
			})

			Expect(found).To(ContainElement(present))
		})
	})

	Context("when the injected exists func reports nothing present", func() {
		It("returns an empty result", func() {
			found := acceptanceharness.ScanCredentialFiles("/fake/home", func(path string) bool {
				return false
			})

			Expect(found).To(BeEmpty())
		})
	})

	Context("when ScanProcessEnv is called with $HOME pointed at a temp dir containing a real .aws/credentials file", func() {
		It("detects the file end-to-end via the real home-directory wiring", func() {
			tmpHome := GinkgoT().TempDir()
			awsDir := filepath.Join(tmpHome, ".aws")
			Expect(os.MkdirAll(awsDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(awsDir, "credentials"), []byte("[default]\naws_access_key_id = placeholder-not-real\n"), 0o600)).To(Succeed())
			GinkgoT().Setenv("HOME", tmpHome)

			result := acceptanceharness.ScanProcessEnv()

			Expect(result.Rejected()).To(BeTrue())
			Expect(result.FoundFiles).To(ContainElement(filepath.Join(tmpHome, ".aws", "credentials")))
		})
	})

	Context("when ScanProcessEnv is called with $HOME pointed at a clean temp dir", func() {
		It("reports no file violations", func() {
			tmpHome := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", tmpHome)

			result := acceptanceharness.ScanProcessEnv()

			Expect(result.FoundFiles).To(BeEmpty())
		})
	})

	Context("when the injected exists func reports only an .aws/config path present (no .aws/credentials)", func() {
		// PR #80 review: ~/.aws/config can also hold static access keys and
		// is consulted by the default AWS SDK provider chain, but the guard
		// previously only scanned .aws/credentials. This spec proves the
		// injectable ScanCredentialFiles path detects .aws/config the same
		// way it detects .aws/credentials.
		It("includes the .aws/config path in FoundFiles", func() {
			home := "/fake/home"
			present := filepath.Join(home, ".aws", "config")

			found := acceptanceharness.ScanCredentialFiles(home, func(path string) bool {
				return path == present
			})

			Expect(found).To(ContainElement(present))
		})
	})

	Context("when ScanProcessEnv is called with $HOME pointed at a temp dir containing only .aws/config (no .aws/credentials)", func() {
		// This is the specific "config only" gap the reviewer called out:
		// an acceptance process with credentials only in .aws/config must
		// still be detected/rejected.
		It("detects the file end-to-end via the real home-directory wiring", func() {
			tmpHome := GinkgoT().TempDir()
			awsDir := filepath.Join(tmpHome, ".aws")
			Expect(os.MkdirAll(awsDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(awsDir, "config"), []byte("[default]\nregion = us-east-1\naws_access_key_id = placeholder-not-real\n"), 0o600)).To(Succeed())
			GinkgoT().Setenv("HOME", tmpHome)

			result := acceptanceharness.ScanProcessEnv()

			Expect(result.Rejected()).To(BeTrue())
			Expect(result.FoundFiles).To(ContainElement(filepath.Join(tmpHome, ".aws", "config")))
		})
	})
})

var _ = Describe("RejectAmbientCredentials", func() {
	Context("when a known ambient-credential variable is set in the real process environment", func() {
		It("returns false and writes a diagnostic naming that variable", func() {
			tmpHome := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", tmpHome)
			GinkgoT().Setenv("GITHUB_TOKEN", "ghp_totallyfake")

			var out bytes.Buffer
			ok := acceptanceharness.RejectAmbientCredentials(&out)

			Expect(ok).To(BeFalse())
			Expect(out.String()).To(ContainSubstring("GITHUB_TOKEN"))
		})
	})

	Context("when the real process environment and default credential-file locations are clean", func() {
		It("returns true and writes nothing", func() {
			tmpHome := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", tmpHome)
			for _, name := range acceptanceharness.AmbientCredentialVars {
				GinkgoT().Setenv(name, "")
				Expect(os.Unsetenv(name)).To(Succeed())
			}

			var out bytes.Buffer
			ok := acceptanceharness.RejectAmbientCredentials(&out)

			Expect(ok).To(BeTrue())
			Expect(out.String()).To(BeEmpty())
		})
	})
})

var _ = Describe("no-egress guard transport", func() {
	Context("when a request targets an allowlisted loopback host", func() {
		It("is allowed through to the injected fake transport", func() {
			fake := &fakeRoundTripper{resp: &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}}
			transport := acceptanceharness.NewGuardedTransport([]string{"127.0.0.1:9999"}, fake)

			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9999/health", nil)
			resp, err := transport.RoundTrip(req)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(fake.called).To(BeTrue())
			Expect(transport.BlockedRequests()).To(BeEmpty())
		})
	})

	Context("when a request targets a disallowed host", func() {
		It("rejects the request before any real network call and records the blocked destination", func() {
			fake := &fakeRoundTripper{resp: &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}}
			transport := acceptanceharness.NewGuardedTransport([]string{"127.0.0.1:9999"}, fake)

			req := httptest.NewRequest(http.MethodGet, "https://api.github.com/repos/lousy-agents/coach", nil)
			resp, err := transport.RoundTrip(req)

			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(fake.called).To(BeFalse(), "the fake transport must never be invoked for a blocked host")
			Expect(transport.BlockedRequests()).To(ContainElement("https://api.github.com/repos/lousy-agents/coach"))
		})
	})

	Context("when a request to a disallowed host has credentials embedded in the URL", func() {
		It("scrubs userinfo and query values from BlockedRequests and the error message, while keeping scheme/host/path", func() {
			fake := &fakeRoundTripper{resp: &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}}
			transport := acceptanceharness.NewGuardedTransport([]string{"127.0.0.1:9999"}, fake)

			req := httptest.NewRequest(http.MethodGet, "https://leaked-user:leaked-pass@api.github.com/repos/x?access_token=super-secret", nil)
			resp, err := transport.RoundTrip(req)

			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(fake.called).To(BeFalse(), "the fake transport must never be invoked for a blocked host")

			blocked := transport.BlockedRequests()
			Expect(blocked).To(HaveLen(1))

			for _, secret := range []string{"leaked-user", "leaked-pass", "super-secret"} {
				Expect(blocked[0]).NotTo(ContainSubstring(secret), "BlockedRequests entry must not leak credentials")
				Expect(err.Error()).NotTo(ContainSubstring(secret), "error message must not leak credentials")
			}

			Expect(blocked[0]).To(ContainSubstring("https://api.github.com/repos/x"), "scrubbed URL must still identify scheme/host/path")
			Expect(err.Error()).To(ContainSubstring("https://api.github.com/repos/x"), "error message must still identify scheme/host/path")
		})
	})
})

// preflightCommandPath is the built acceptance-guard-preflight binary path,
// set once by the BeforeSuite below and reused by every command-boundary
// spec in this file.
var preflightCommandPath string

var _ = BeforeSuite(func() {
	directory, err := os.MkdirTemp("", "acceptanceharness-guard-preflight-cmd-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	preflightCommandPath = filepath.Join(directory, "acceptance-guard-preflight")
	build := exec.Command("go", "build", "-o", preflightCommandPath, "github.com/lousy-agents/coach/cmd/acceptance-guard-preflight")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building acceptance-guard-preflight: %s", output)
})

// cleanEnvironForPreflightCmd returns the current process environment with
// every known ambient-credential variable stripped and HOME repointed at
// home, so a spawned acceptance-guard-preflight subprocess never observes
// this test runner's own real ambient environment (or a real
// ~/.aws/credentials on the host running this suite) regardless of what's
// actually present. Mirrors cmd/acceptance-guard-preflight/acceptance_test.go's
// cleanEnviron helper, duplicated locally so this file does not need to
// import that internal test helper across package boundaries.
func cleanEnvironForPreflightCmd(home string) []string {
	ambient := make(map[string]bool, len(acceptanceharness.AmbientCredentialVars))
	for _, name := range acceptanceharness.AmbientCredentialVars {
		ambient[name] = true
	}

	var out []string
	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || ambient[name] || name == "HOME" {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "HOME="+home)
}

var _ = Describe("command-boundary: acceptance-guard-preflight binary (new credential categories)", func() {
	Context("when run with a new OAuth or model-provider ambient-credential variable set", func() {
		DescribeTable("exits non-zero and writes a diagnostic naming that variable to stderr",
			func(name, value string) {
				home := GinkgoT().TempDir()
				cmd := exec.Command(preflightCommandPath)
				cmd.Env = append(cleanEnvironForPreflightCmd(home), name+"="+value)
				var stderr strings.Builder
				cmd.Stderr = &stderr

				err := cmd.Run()

				Expect(err).To(HaveOccurred())
				var exitErr *exec.ExitError
				Expect(err).To(BeAssignableToTypeOf(exitErr))
				Expect(stderr.String()).To(ContainSubstring(name))
			},
			Entry("GitHub OAuth client secret", "GITHUB_CLIENT_SECRET", "fake-oauth-client-secret"),
			Entry("OpenAI API key", "OPENAI_API_KEY", "sk-fake-openai-key"),
		)
	})

	Context("when run with only an .aws/config file present (no .aws/credentials, no env var)", func() {
		// PR #80 review, second round: the preflight command boundary must
		// also reject when the only ambient-credential source is
		// ~/.aws/config.
		It("exits non-zero and writes a diagnostic naming the .aws/config file to stderr", func() {
			home := GinkgoT().TempDir()
			awsDir := filepath.Join(home, ".aws")
			Expect(os.MkdirAll(awsDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(awsDir, "config"), []byte("[default]\nregion = us-east-1\naws_access_key_id = placeholder-not-real\n"), 0o600)).To(Succeed())

			cmd := exec.Command(preflightCommandPath)
			cmd.Env = cleanEnvironForPreflightCmd(home)
			var stderr strings.Builder
			cmd.Stderr = &stderr

			err := cmd.Run()

			Expect(err).To(HaveOccurred())
			var exitErr *exec.ExitError
			Expect(err).To(BeAssignableToTypeOf(exitErr))
			Expect(stderr.String()).To(ContainSubstring(filepath.Join(home, ".aws", "config")))
		})
	})
})
