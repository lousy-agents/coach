package redisstream

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// reclaimExpired looks for one pendingClaim that is either readyForClaim
// (a retryable Nack already incremented Attempt and re-tokened it; see
// pendingClaim's doc comment) or whose claimAfter has elapsed per
// q.clock.Now(), and if found, returns it -- for the expiry case,
// replacing it in place with a new token and an incremented attempt count.
// Either path invalidates the old Token, per TaskQueue.Complete/Nack's
// stale-token contract, without touching the underlying Watermill
// message's Ack/Nack channels (see pendingClaim's doc comment).
func (q *Queue) reclaimExpired() (queue.Claim, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock.Now()
	for token, pc := range q.pending {
		if pc.readyForClaim {
			pc.readyForClaim = false
			return queue.Claim{TaskID: pc.taskID, Attempt: pc.attempt, Token: pc.token}, true
		}
		if now.Sub(pc.claimedAt) < q.claimAfter {
			continue
		}
		delete(q.pending, token)
		pc.attempt++
		pc.token = watermill.NewUUID()
		pc.claimedAt = now
		q.pending[pc.token] = pc
		return queue.Claim{TaskID: pc.taskID, Attempt: pc.attempt, Token: pc.token}, true
	}
	return queue.Claim{}, false
}

// Nack reports claim's task attempt failed. A retryable failure
// (permanent=false) makes the task claimable again with Attempt
// incremented. A permanent failure (permanent=true) acknowledges the
// underlying message (ADR-006 rule 5) and republishes the task onto the
// poison-task destination stream; PoisonTasks reads that stream back.
// Nack fails under the same stale-token condition as Complete.
func (q *Queue) Nack(ctx context.Context, claim queue.Claim, permanent bool) error {
	pc, ok := q.takePending(claim)
	if !ok {
		return fmt.Errorf("redisstream: nack: claim token invalid or expired for task %q", claim.TaskID)
	}

	if permanent {

		if err := q.publishPoison(ctx, pc.taskID, pc.msg.Payload); err != nil {
			q.mu.Lock()
			q.pending[pc.token] = pc
			q.mu.Unlock()
			return err
		}
		pc.msg.Ack()
		return nil
	}

	pc.attempt++
	pc.token = watermill.NewUUID()
	pc.claimedAt = q.clock.Now()

	pc.readyForClaim = true

	q.mu.Lock()
	q.pending[pc.token] = pc
	q.mu.Unlock()

	return nil
}

// PoisonTasks returns every task a permanent Nack has routed to the
// poison-task destination stream, oldest first. It is not part of
// queue.TaskQueue; internal/acceptanceharness/queueconformance.Queue
// requires it so both this package's own tests and the shared
// conformance suite can assert the poison destination actually received a
// task.
func (q *Queue) PoisonTasks(ctx context.Context) ([]queue.Task, error) {
	entries, err := q.client.XRange(ctx, q.poisonStream, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("redisstream: reading poison-task destination %q: %w", q.poisonStream, err)
	}

	tasks := make([]queue.Task, 0, len(entries))
	for _, entry := range entries {
		msg, err := q.unmarshaller.Unmarshal(entry.Values)
		if err != nil {
			return nil, fmt.Errorf("redisstream: decoding poison-task destination entry %s: %w", entry.ID, err)
		}
		tasks = append(tasks, queue.Task{ID: msg.Metadata.Get(taskIDMetadataKey), Payload: msg.Payload})
	}
	return tasks, nil
}

// takePending removes and returns the pendingClaim matching claim's
// Token, or ok=false if no such claim is currently outstanding (already
// completed, poisoned, or superseded by a reclaim) -- the stale-token
// condition Complete and Nack must both fail under.
func (q *Queue) takePending(claim queue.Claim) (*pendingClaim, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pc, ok := q.pending[claim.Token]
	if !ok || pc.taskID != claim.TaskID {
		return nil, false
	}
	delete(q.pending, claim.Token)
	return pc, true
}
