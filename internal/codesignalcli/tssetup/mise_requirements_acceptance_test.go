package tssetup

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func ambientNpmConfigNames() []string {
	var names []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToLower(name), "npm_config_") {
			names = append(names, name)
		}
	}
	return names
}

func ambientNpmUserConfigNamingAnUnreachableRegistry() {
	decoy := GinkgoT().TempDir() + "/ambient.npmrc"
	Expect(os.WriteFile(decoy, []byte("registry=http://127.0.0.1:1/\n"), 0o644)).To(Succeed())
	GinkgoT().Setenv("NPM_CONFIG_USERCONFIG", decoy)
}

var _ = Describe("the real-mise specs that serve their own loopback npm registry do not depend on the host's npm configuration (#891)", func() {
	When("an ambient npm user config names a registry nothing listens on", func() {
		BeforeEach(ambientNpmUserConfigNamingAnUnreachableRegistry)

		It("still installs the sentinel for the insulated-working-directory positive control, which runs mise itself and so inherits the ambient environment", neverExecutesBecauseRealInstallRunsFrom)
	})

	It("removes every ambient npm_config_* variable once a spec points npm at its loopback registry", func() {
		GinkgoT().Setenv("NPM_CONFIG_USERCONFIG", "/nonexistent/.npmrc")
		GinkgoT().Setenv("npm_config_registry", "http://127.0.0.1:1/")
		freshMiseInstallEnv()

		writeHomeNpmrcRegistry("http://127.0.0.1:2/")

		Expect(ambientNpmConfigNames()).To(BeEmpty())
	})

	It("leaves the ambient proxy settings in place for a spec that installs from the public registry", func() {
		GinkgoT().Setenv("npm_config_https_proxy", "http://127.0.0.1:3/")

		freshMiseInstallEnv()

		Expect(os.Getenv("npm_config_https_proxy")).To(Equal("http://127.0.0.1:3/"))
	})
})

var _ = Describe("a real-mise spec that cannot run says why, and cannot silently stop proving anything where the toolchain is promised (#891)", func() {
	cause := errors.New("exec: \"mise\": executable file not found in $PATH")

	DescribeTable("what a missing requirement does",
		func(requirement string, cause error, required bool, wantSkip, wantFailure bool) {
			skipReason, failure := outcomeWhenRequirementMissing(requirement, cause, required)

			Expect(skipReason != "").To(Equal(wantSkip), "skip reason %q", skipReason)
			Expect(failure != "").To(Equal(wantFailure), "failure %q", failure)
			if wantSkip {
				Expect(skipReason).To(And(ContainSubstring(requirement), ContainSubstring(cause.Error())))
			}
			if wantFailure {
				Expect(failure).To(And(ContainSubstring(requirement), ContainSubstring(cause.Error())))
			}
		},
		Entry("is present, not required: neither", "mise", nil, false, false, false),
		Entry("is present, required: neither", "mise", nil, true, false, false),
		Entry("is missing, not required: skips, naming it and why", "mise", cause, false, true, false),
		Entry("is missing, required: fails, naming it and why", "mise", cause, true, false, true),
	)

	DescribeTable("when the toolchain counts as promised",
		func(setting string, set bool, want bool) {
			if set {
				GinkgoT().Setenv(realToolchainRequiredVariable, setting)
			} else {
				GinkgoT().Setenv(realToolchainRequiredVariable, "")
				Expect(os.Unsetenv(realToolchainRequiredVariable)).To(Succeed())
			}

			Expect(realToolchainRequired()).To(Equal(want))
		},
		Entry("the variable is unset: not promised", "", false, false),
		Entry("the variable is empty: not promised", "", true, false),
		Entry("the variable is 1: promised", "1", true, true),
	)

	When("the public npm registry is the requirement", func() {
		It("is reachable: no error", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
			DeferCleanup(server.Close)

			Expect(probeRegistry(server.Client(), server.URL)).To(Succeed())
		})

		It("refuses the connection: an error naming the registry", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
			closed := server.URL
			server.Close()

			Expect(probeRegistry(&http.Client{Timeout: time.Second}, closed)).To(MatchError(ContainSubstring(closed)))
		})

		It("answers too slowly: an error naming the registry", func() {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { time.Sleep(300 * time.Millisecond) }))
			DeferCleanup(server.Close)

			Expect(probeRegistry(&http.Client{Timeout: 50 * time.Millisecond}, server.URL)).To(MatchError(ContainSubstring(server.URL)))
		})

		It("answers with a server error: an error naming the registry and the status", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
			DeferCleanup(server.Close)

			Expect(probeRegistry(server.Client(), server.URL)).To(MatchError(And(ContainSubstring(server.URL), ContainSubstring("502"))))
		})
	})
})
