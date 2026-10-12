package redisstream

import (
	"context"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// Claim first reclaims any pending claim whose ClaimAfter has elapsed
// (per the injected Clock), and only if none did, waits up to
// claimPollWindow for a newly delivered message. ok=false means neither
// happened before the wait window (or ctx) elapsed.
func (q *Queue) Claim(ctx context.Context) (queue.Claim, bool, error) {
	if claim, ok := q.reclaimExpired(); ok {
		return claim, true, nil
	}

	timer := time.NewTimer(claimPollWindow)
	defer timer.Stop()

	select {
	case msg, open := <-q.messages:
		if !open {
			return queue.Claim{}, false, fmt.Errorf("redisstream: subscriber channel closed")
		}
		return q.trackNewClaim(msg), true, nil
	case <-timer.C:
		return queue.Claim{}, false, nil
	case <-ctx.Done():
		return queue.Claim{}, false, ctx.Err()
	}
}

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

// trackNewClaim records a freshly delivered Watermill message as a new
// pendingClaim (attempt 0) and returns its Claim.
func (q *Queue) trackNewClaim(msg *message.Message) queue.Claim {
	taskID := msg.Metadata.Get(taskIDMetadataKey)
	token := watermill.NewUUID()

	q.mu.Lock()
	q.pending[token] = &pendingClaim{
		taskID:    taskID,
		attempt:   0,
		token:     token,
		claimedAt: q.clock.Now(),
		msg:       msg,
	}
	q.mu.Unlock()

	return queue.Claim{TaskID: taskID, Attempt: 0, Token: token}
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
