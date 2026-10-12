package sqs_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	sqsqueue "github.com/lousy-agents/coach/internal/coachapi/queue/sqs"
)

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
		return newLocalStackConformanceQueue(tb, clock, endpoint, queueURL)
	})
}

// newLocalStackConformanceQueue builds the real adapter under test against
// the LocalStack queue at queueURL.
func newLocalStackConformanceQueue(tb testing.TB, clock acceptanceharness.Clock, endpoint, queueURL string) queueconformance.Queue {
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
