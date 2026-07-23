package sqs_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	sqsqueue "github.com/lousy-agents/coach/internal/coachapi/queue/sqs"
)

// localstackReadyTimeout bounds how long TestSQSQueueConformanceAcceptance
// waits for a freshly started LocalStack container to report its SQS
// service healthy before skipping (rather than failing) the whole test:
// this sandbox and slow CI runners both need to be tolerated, and a hung
// wait is worse than a graceful skip.
const localstackReadyTimeout = 45 * time.Second

// conformanceAdapter bridges *sqsqueue.Queue (which implements
// internal/coachapi/queue.TaskQueue, the real port) to
// queueconformance.Queue: the two interfaces are structurally identical but
// declared with distinct named Task/Claim types, so a thin conversion layer
// is required to run the shared black-box suite against the real adapter.
type conformanceAdapter struct {
	q *sqsqueue.Queue
}

func (a conformanceAdapter) Enqueue(ctx context.Context, task queueconformance.Task) error {
	return a.q.Enqueue(ctx, queue.Task{ID: task.ID, Payload: task.Payload})
}

func (a conformanceAdapter) Claim(ctx context.Context) (queueconformance.Claim, bool, error) {
	claim, ok, err := a.q.Claim(ctx)
	return queueconformance.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token}, ok, err
}

func (a conformanceAdapter) Complete(ctx context.Context, claim queueconformance.Claim) error {
	return a.q.Complete(ctx, queue.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token})
}

func (a conformanceAdapter) Nack(ctx context.Context, claim queueconformance.Claim, permanent bool) error {
	return a.q.Nack(ctx, queue.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token}, permanent)
}

func (a conformanceAdapter) PoisonTasks(ctx context.Context) ([]queueconformance.Task, error) {
	tasks, err := a.q.PoisonTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]queueconformance.Task, len(tasks))
	for i, task := range tasks {
		out[i] = queueconformance.Task{ID: task.ID, Payload: task.Payload}
	}
	return out, nil
}

// TestSQSQueueConformanceAcceptance is this package's acceptance test: it
// runs internal/acceptanceharness/queueconformance's black-box suite
// against a real *sqsqueue.Queue backed by a throwaway LocalStack SQS
// queue. It skips gracefully (never fails) whenever Docker is unusable
// here: not on PATH, daemon unreachable, or the LocalStack container fails
// to start or become healthy within localstackReadyTimeout -- matching
// internal/acceptanceharness/thinproof/compose_acceptance_test.go's
// convention, extended to also cover a reachable-but-broken daemon.
//
// Credential safety: this test never reads ambient AWS credentials. It
// pins a hardcoded, obviously-fake static access key/secret at the
// LocalStack endpoint only -- see the package doc comment's "Credential
// safety" section.
func TestSQSQueueConformanceAcceptance(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH; skipping the LocalStack-backed SQS conformance suite")
	}

	endpoint, containerID, ok := startLocalStack(t)
	if !ok {
		return // already skipped with a reason by startLocalStack
	}
	t.Cleanup(func() {
		stop := exec.Command("docker", "rm", "-f", containerID)
		stop.CombinedOutput() //nolint:errcheck // best-effort cleanup
	})

	if !waitForLocalStackReady(t, endpoint) {
		t.Skip("LocalStack did not report a healthy SQS service within the bounded wait; skipping")
		return
	}

	suffix := randomSuffix(t)
	queueName := "coach-sqs-conformance-" + suffix
	queueURL, ok := createLocalStackQueue(t, endpoint, queueName)
	if !ok {
		return
	}
	t.Cleanup(func() {
		deleteLocalStackQueue(endpoint, queueURL)
		deleteLocalStackQueue(endpoint, queueURL+"-poison")
	})

	queueconformance.Run(t, func(tb testing.TB, clock acceptanceharness.Clock) queueconformance.Queue {
		cfg := sqsqueue.Config{
			Region:            "us-east-1",
			QueueURL:          queueURL,
			VisibilityTimeout: time.Minute,
			Endpoint:          endpoint,
			// Explicit, hardcoded, obviously-fake credentials pinned at the
			// LocalStack endpoint -- never the ambient AWS credential
			// chain (see the package doc comment's "Credential safety"
			// section).
			Credentials: credentials.NewStaticCredentialsProvider("localstack-fake-access-key", "localstack-fake-secret-key", ""),
		}
		q, err := sqsqueue.NewQueue(context.Background(), cfg, clock)
		if err != nil {
			tb.Fatalf("sqs.NewQueue: %v", err)
		}
		return conformanceAdapter{q: q}
	})
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
		"localstack/localstack",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker run localstack/localstack failed (Docker daemon likely unreachable in this environment); skipping: %v\n%s", err, out)
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

// waitForLocalStackReady polls LocalStack's health endpoint until the SQS
// service reports "available"/"running", or localstackReadyTimeout elapses.
func waitForLocalStackReady(t *testing.T, endpoint string) bool {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(localstackReadyTimeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(endpoint + "/_localstack/health")
		if err == nil {
			var health struct {
				Services map[string]string `json:"services"`
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil && json.Unmarshal(body, &health) == nil {
				if status := health.Services["sqs"]; status == "available" || status == "running" {
					return true
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

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
	// LocalStack's SQS query-protocol endpoint accepts unsigned requests
	// with any Authorization header shape for local testing.
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

// extractQueueURL pulls <QueueUrl>...</QueueUrl> out of an SQS
// CreateQueueResponse XML body.
func extractQueueURL(body string) string {
	const open, close = "<QueueUrl>", "</QueueUrl>"
	start := strings.Index(body, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(body[start:], close)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
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
