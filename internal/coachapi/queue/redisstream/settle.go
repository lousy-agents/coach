package redisstream

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// Complete acknowledges claim's task attempt as durably finished. It
// fails if claim.Token has been invalidated by a reclaim.
func (q *Queue) Complete(ctx context.Context, claim queue.Claim) error {
	pc, ok := q.takePending(claim)
	if !ok {
		return fmt.Errorf("redisstream: complete: claim token invalid or expired for task %q", claim.TaskID)
	}
	pc.msg.Ack()
	return nil
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
		// Publish to the poison destination before acking the source
		// message: if publishPoison fails, the source message stays
		// pending (unacked) rather than being silently dropped, so a
		// crash/retry can still recover the task instead of losing it.
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
	// readyForClaim makes the retried task immediately reclaimable by the
	// next Claim (see pendingClaim's doc comment), per TaskQueue's
	// immediate-availability-after-retryable-Nack contract, instead of
	// waiting out a fresh claimAfter window.
	pc.readyForClaim = true

	q.mu.Lock()
	q.pending[pc.token] = pc
	q.mu.Unlock()

	return nil
}
