package tssetup

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// realToolchainRequiredVariable is set by the CI job that installs mise, npm and a route to the public registry for
// these specs. distro-git runs the same package without npm, so the CI environment alone cannot say the tools are promised.
const realToolchainRequiredVariable = "COACH_REQUIRE_REAL_MISE"

func realToolchainRequired() bool { return os.Getenv(realToolchainRequiredVariable) != "" }

// outcomeWhenRequirementMissing decides what a real-mise spec does when something it needs is absent: without the variable it
// skips and says why; with it, it fails, because a skipped control proves nothing and nothing else would notice.
func outcomeWhenRequirementMissing(requirement string, cause error, required bool) (skipReason, failure string) {
	if cause == nil {
		return "", ""
	}
	if required {
		return "", fmt.Sprintf("%s is required here and is not available (%s)", requirement, cause)
	}
	return fmt.Sprintf("%s is not available (%s)", requirement, cause), ""
}

const registryProbeTimeout = 3 * time.Second

var publicNpmRegistryProbeURL = "https://registry.npmjs.org/typescript"

func requireToolOnPath(name string) {
	_, err := exec.LookPath(name)
	requireOrSkip(name, err)
}

func requirePublicNpmRegistry() {
	skipReason, failure := registryProbeOutcome(probeRegistry(&http.Client{Timeout: registryProbeTimeout}, publicNpmRegistryProbeURL), realToolchainRequired())
	Expect(failure).To(BeEmpty())
	if skipReason != "" {
		Skip(skipReason)
	}
}

func requireOrSkip(requirement string, cause error) {
	skipReason, failure := outcomeWhenRequirementMissing(requirement, cause, realToolchainRequired())
	Expect(failure).To(BeEmpty())
	if skipReason != "" {
		Skip(skipReason)
	}
}

// registryProbeMisconfigured is a 4xx answer other than 429: the probe URL is wrong or the registry does not serve it, so
// no later run would fix it, and skipping would hide it.
type registryProbeMisconfigured struct{ error }

func registryProbeOutcome(probeErr error, required bool) (skipReason, failure string) {
	var misconfigured *registryProbeMisconfigured
	if errors.As(probeErr, &misconfigured) {
		return "", probeErr.Error()
	}
	return outcomeWhenRequirementMissing("the public npm registry", probeErr, required)
}

// probeRegistry reports why registryURL cannot be installed from now, with the client's timeout and the process's proxy
// settings: a transport error, a 429 or a 5xx is unavailable, any other 4xx is a misconfigured probe, and a 2xx or 3xx is reachable.
func probeRegistry(client *http.Client, registryURL string) error {
	response, err := client.Get(registryURL)
	if err != nil {
		return fmt.Errorf("probe %s: %w", registryURL, err)
	}
	defer response.Body.Close()
	status := response.StatusCode
	probeErr := fmt.Errorf("probe %s: status %d", registryURL, status)
	switch {
	case status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		return probeErr
	case status >= http.StatusBadRequest:
		return &registryProbeMisconfigured{probeErr}
	}
	return nil
}
