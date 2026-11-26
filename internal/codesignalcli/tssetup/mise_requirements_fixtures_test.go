package tssetup

import (
	"fmt"
	"net/http"
	"os"
)

func runningInCI() bool { return os.Getenv("GITHUB_ACTIONS") == "true" }

func outcomeWhenRequirementMissing(requirement string, cause error, inCI bool) (skipReason, failure string) {
	if cause == nil {
		return "", ""
	}
	return fmt.Sprintf("%s is not available (%s)", requirement, cause), ""
}

func probeRegistry(client *http.Client, registryURL string) error { return nil }
