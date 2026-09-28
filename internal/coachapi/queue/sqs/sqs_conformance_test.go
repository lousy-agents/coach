package sqs_test

import (
	"context"

	"encoding/json"
	"io"
	"net/http"
	"os/exec"

	"testing"
	"time"

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
		return
	}
	t.Cleanup(func() {
		stop := exec.Command("docker", "rm", "-f", containerID)
		stop.CombinedOutput()
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
		return body_sqsConformanceTest_81(tb, clock, endpoint, queueURL)
	})
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
