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

func runningInCI() bool { return os.Getenv("GITHUB_ACTIONS") == "true" }

// outcomeWhenRequirementMissing decides what a real-mise spec does when something it needs is absent: outside CI it
// skips and says why; in CI it fails, because a skipped control proves nothing and nothing else would notice.
func outcomeWhenRequirementMissing(requirement string, cause error, inCI bool) (skipReason, failure string) {
	if cause == nil {
		return "", ""
	}
	if inCI {
		return "", fmt.Sprintf("%s is required in CI and is not available (%s)", requirement, cause)
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
	skipReason, failure := outcomeWhenRequirementMissing(requirement, cause, runningInCI())
	Expect(failure).To(BeEmpty())
	if skipReason != "" {
		Skip(skipReason)
	}
}

// probeRegistry reports why registryURL cannot be reached with the client's timeout and the process's proxy settings.
func probeRegistry(client *http.Client, registryURL string) error {
	response, err := client.Get(registryURL)
	if err != nil {
		return fmt.Errorf("probe %s: %w", registryURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("probe %s: status %d", registryURL, response.StatusCode)
	}
	return nil
}
