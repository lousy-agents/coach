package sqs_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// localstackReadyTimeout bounds how long TestSQSQueueConformanceAcceptance
// waits for a freshly started LocalStack container to report its SQS
// service healthy before skipping (rather than failing) the whole test:
// this sandbox and slow CI runners both need to be tolerated, and a hung
// wait is worse than a graceful skip.
const localstackReadyTimeout = 45 * time.Second

// startLocalStack starts a throwaway LocalStack container with only the
// SQS service enabled, publishing its edge port (4566) to a
// Docker-assigned host port. ok=false means the whole test has already
// been skipped (Docker daemon unreachable or the container failed to
// start); callers must return immediately without further cleanup beyond
// what startLocalStack itself already registered.
func startLocalStack(t *testing.T) (endpoint, containerID string, ok bool) {
	t.Helper()

	runCmd := exec.Command("docker", "run", "--rm", "-d",
		"-p", "0:4566",
		"-e", "SERVICES=sqs",
		// Pinned (not :latest) so CI doesn't get a surprise behavior change
		// from an upstream LocalStack release, mirroring redisstream's
		// pinned redis:7-alpine.
		"localstack/localstack:3.8",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker run localstack/localstack:3.8 failed (Docker daemon likely unreachable in this environment); skipping: %v\n%s", err, out)
		return "", "", false
	}
	containerID = strings.TrimSpace(string(out))

	portCmd := exec.Command("docker", "port", containerID, "4566/tcp")
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		exec.Command("docker", "rm", "-f", containerID).Run() //nolint:errcheck
		t.Skipf("docker port lookup for the LocalStack container failed; skipping: %v\n%s", err, portOut)
		return "", "", false
	}

	hostPort := parseHostPort(string(portOut))
	if hostPort == "" {
		exec.Command("docker", "rm", "-f", containerID).Run() //nolint:errcheck
		t.Skipf("could not parse a host port from `docker port` output %q; skipping", portOut)
		return "", "", false
	}

	return "http://127.0.0.1:" + hostPort, containerID, true
}

// waitForLocalStackReady polls LocalStack's health endpoint until the SQS
// service reports "available"/"running", or localstackReadyTimeout elapses.
func waitForLocalStackReady(t *testing.T, endpoint string) bool {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(localstackReadyTimeout)
	for time.Now().Before(deadline) {
		if localStackSQSHealthy(client, endpoint) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// localStackSQSHealthy reports whether one health probe sees LocalStack's
// SQS service "available" or "running".
func localStackSQSHealthy(client *http.Client, endpoint string) bool {
	resp, err := client.Get(endpoint + "/_localstack/health")
	if err != nil {
		return false
	}
	var health struct {
		Services map[string]string `json:"services"`
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil || json.Unmarshal(body, &health) != nil {
		return false
	}
	status := health.Services["sqs"]
	return status == "available" || status == "running"
}

// parseHostPort extracts the host port from `docker port <id> 4566/tcp`
// output, which looks like "0.0.0.0:32768\n[::]:32768\n".
func parseHostPort(portOutput string) string {
	for _, line := range strings.Split(strings.TrimSpace(portOutput), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.LastIndex(line, ":")
		if idx < 0 || idx == len(line)-1 {
			continue
		}
		return line[idx+1:]
	}
	return ""
}
