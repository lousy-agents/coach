package redisstream_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

// startRedisContainer starts a throwaway `redis:7-alpine` container on a
// dynamically assigned host port, following this repo's Docker-CLI (not
// testcontainers-go) convention for discovering that port (see
// internal/acceptanceharness/thinproof/compose_acceptance_test.go). Any
// failure -- docker missing, daemon unreachable, container failing to
// start -- calls t.Skip rather than t.Fatal, since Docker's daemon is not
// guaranteed reachable in every environment this suite runs in.
func startRedisContainer(t *testing.T) (address string) {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH; skipping the Redis Streams conformance suite")
	}

	runCmd := exec.Command("docker", "run", "--rm", "-d", "-p", "0:6379", "redis:7-alpine")
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker run redis:7-alpine failed (daemon likely unreachable in this environment): %v\n%s", err, out)
	}
	containerID := strings.TrimSpace(string(out))
	if containerID == "" {
		t.Skip("docker run redis:7-alpine produced no container id; skipping")
	}

	t.Cleanup(func() {
		stopCmd := exec.Command("docker", "stop", containerID)
		stopCmd.CombinedOutput() //nolint:errcheck // best-effort cleanup
	})

	portCmd := exec.Command("docker", "port", containerID, "6379/tcp")
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker port %s failed: %v\n%s", containerID, err, portOut)
	}

	// `docker port` prints e.g. "0.0.0.0:54321\n" (and, on some hosts, an
	// additional "[::]:54321" IPv6 line); the first line's port is enough.
	firstLine := strings.SplitN(strings.TrimSpace(string(portOut)), "\n", 2)[0]
	hostPort := firstLine[strings.LastIndex(firstLine, ":")+1:]
	if _, err := strconv.Atoi(hostPort); err != nil {
		t.Skipf("could not parse host port from `docker port` output %q: %v", portOut, err)
	}

	address = "127.0.0.1:" + hostPort

	if !waitForRedisReady(address, 30*time.Second) {
		t.Skipf("redis:7-alpine at %s did not become ready in time; skipping", address)
	}

	return address
}

// waitForRedisReady polls NewQueue's own Ping-based connectivity check
// until it succeeds or timeout elapses, since a freshly started container
// is not guaranteed to accept connections immediately.
func waitForRedisReady(address string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		q, err := redisstream.NewQueue(redisstream.Config{
			Address:       address,
			Stream:        "readiness-check-" + watermill.NewUUID(),
			ConsumerGroup: "readiness-check",
			ClaimAfter:    time.Minute,
			DialTimeout:   500 * time.Millisecond,
		}, acceptanceharness.RealClock{})
		if err == nil {
			_ = q.Close()
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
