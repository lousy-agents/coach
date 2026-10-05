package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	configTestSigningKey = "test-signing-secret-at-least-32-bytes!!"
	configTestIssuer     = "https://coach-api.test"
	configTestHTTPAddr   = ":8080"

	configTestGitHubAppID   = "123"
	configTestGitHubPrivKey = "test-private-key-pem-contents"
	configTestRedisAddr     = "127.0.0.1:6379"
)

// setEnv sets key=value for the duration of the current spec via
// GinkgoT().Setenv, which restores the prior value (or unsets it) in
// cleanup, so specs never leak env vars into one another.
func setEnv(key, value string) {
	GinkgoHelper()
	GinkgoT().Setenv(key, value)
}

// clearEnv unsets each key for the duration of the current spec,
// restoring whatever value (or absence) preceded it once the spec ends.
func clearEnv(keys ...string) {
	GinkgoHelper()
	for _, key := range keys {
		key := key
		restoreEnvAfterSpec(key)
		Expect(os.Unsetenv(key)).To(Succeed())
	}
}

// restoreEnvAfterSpec puts key back to its current value when the spec
// ends, if key is set now.
func restoreEnvAfterSpec(key string) {
	if v, ok := os.LookupEnv(key); ok {
		DeferCleanup(func() { Expect(os.Setenv(key, v)).To(Succeed()) })
	}
}

// setValidConfigEnv sets every var loadConfigFromEnv requires, plus clears
// the optional ones this suite cares about isolating (COACH_AUTH_TEST_MINT,
// GitHub OAuth), so each spec can override exactly the one var under test.
func setValidConfigEnv() {
	GinkgoHelper()
	setEnv("COACH_JWT_SIGNING_KEY", configTestSigningKey)
	setEnv("COACH_JWT_ISSUER", configTestIssuer)
	setEnv("COACH_HTTP_ADDR", configTestHTTPAddr)
	clearEnv("COACH_AUTH_TEST_MINT", "COACH_JWT_TOKEN_TTL",
		"COACH_GITHUB_OAUTH_CLIENT_ID", "COACH_GITHUB_OAUTH_CLIENT_SECRET",
		"COACH_GITHUB_OAUTH_REDIRECT_URI", "COACH_GITHUB_OAUTH_BASE_URL",
		"COACH_GITHUB_OAUTH_API_BASE_URL")
}

// setValidInfraConfigEnv sets every var loadInfraConfigFromEnv requires.
func setValidInfraConfigEnv() {
	GinkgoHelper()
	setEnv("COACH_GITHUB_APP_ID", configTestGitHubAppID)
	setEnv("COACH_GITHUB_APP_PRIVATE_KEY", configTestGitHubPrivKey)
	setEnv("COACH_REDIS_ADDR", configTestRedisAddr)
	clearEnv("COACH_GITHUB_APP_PRIVATE_KEY_PATH", "COACH_AUTHZ_BYPASS_OWNER", "COACH_AUTHZ_BYPASS_REPO")
}
