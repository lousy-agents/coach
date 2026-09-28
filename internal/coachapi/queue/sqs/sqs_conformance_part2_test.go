package sqs_test

import (
	"crypto/rand"
	"encoding/hex"

	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// createLocalStackQueue creates a fresh SQS queue named queueName against
// LocalStack via a plain HTTP call (not the sqs package under test, so this
// setup step exercises an independent path from what's under test) and
// returns its queue URL.
func createLocalStackQueue(t *testing.T, endpoint, queueName string) (string, bool) {
	t.Helper()

	form := "Action=CreateQueue&QueueName=" + queueName + "&Version=2012-11-05"
	req, err := http.NewRequest(http.MethodPost, endpoint+"/", strings.NewReader(form))
	if err != nil {
		t.Skipf("building CreateQueue request failed; skipping: %v", err)
		return "", false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=localstack-fake-access-key/20260101/us-east-1/sqs/aws4_request")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("CreateQueue against LocalStack failed; skipping: %v", err)
		return "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Skipf("CreateQueue against LocalStack returned %d; skipping: %s", resp.StatusCode, body)
		return "", false
	}

	queueURL := extractQueueURL(string(body))
	if queueURL == "" {
		t.Skipf("could not parse a queue URL from CreateQueue response; skipping: %s", body)
		return "", false
	}
	return queueURL, true
}

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
		exec.Command("docker", "rm", "-f", containerID).Run()
		t.Skipf("docker port lookup for the LocalStack container failed; skipping: %v\n%s", err, portOut)
		return "", "", false
	}

	hostPort := parseHostPort(string(portOut))
	if hostPort == "" {
		exec.Command("docker", "rm", "-f", containerID).Run()
		t.Skipf("could not parse a host port from `docker port` output %q; skipping", portOut)
		return "", "", false
	}

	return "http://127.0.0.1:" + hostPort, containerID, true
}

func deleteLocalStackQueue(endpoint, queueURL string) {
	if queueURL == "" {
		return
	}
	form := "Action=DeleteQueue&QueueUrl=" + queueURL + "&Version=2012-11-05"
	req, err := http.NewRequest(http.MethodPost, endpoint+"/", strings.NewReader(form))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generating random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}
