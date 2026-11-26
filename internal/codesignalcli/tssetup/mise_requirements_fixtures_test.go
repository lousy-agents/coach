package tssetup

import (
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

const (
	registryProbeTimeout      = 3 * time.Second
	publicNpmRegistryProbeURL = "https://registry.npmjs.org/typescript"
)

func requireToolOnPath(name string) {
	_, err := exec.LookPath(name)
	requireOrSkip(name, err)
}

func requireOrSkip(requirement string, cause error) {
	skipReason, failure := outcomeWhenRequirementMissing(requirement, cause, realToolchainRequired())
	Expect(failure).To(BeEmpty())
	if skipReason != "" {
		Skip(skipReason)
	}
}

// probeRegistry treats any 4xx or 5xx as unavailable: a 429 or a mirror that lacks the package would otherwise send the
// real install on to fail as a false red. It reports why registryURL cannot be reached with the client's timeout and the process's proxy settings.
func probeRegistry(client *http.Client, registryURL string) error {
	response, err := client.Get(registryURL)
	if err != nil {
		return fmt.Errorf("probe %s: %w", registryURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("probe %s: status %d", registryURL, response.StatusCode)
	}
	return nil
}
