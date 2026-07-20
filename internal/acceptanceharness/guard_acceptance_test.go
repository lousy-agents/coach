package acceptanceharness_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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
