package acceptanceharness_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func body_guardAcceptanceTest_unsetsAPreviouslySetAmbientCredentialVarSoItIsNo_104() {
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
}

func body_guardAcceptanceTest_returnsTrueAndWritesNothing_241() {
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
}

func body_guardAcceptanceTest_scrubsUserinfoAndQueryValuesFromBlockedRequestsA_290() {
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
}
