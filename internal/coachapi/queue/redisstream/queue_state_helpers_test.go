package redisstream

import (
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// newTestQueue builds a Queue whose state machine (pending map, clock,
// claimAfter) can be exercised directly, with no Redis client/publisher/
// subscriber involved -- Claim/Nack/Complete's reclaim and token
// invalidation logic never touches those fields.
func newTestQueue(clock acceptanceharness.Clock, claimAfter time.Duration) *Queue {
	return &Queue{
		claimAfter: claimAfter,
		clock:      clock,
		pending:    make(map[string]*pendingClaim),
	}
}

func (q *Queue) seedClaim(taskID, token string, claimedAt time.Time) {
	pc := &pendingClaim{
		taskID:    taskID,
		attempt:   0,
		token:     token,
		claimedAt: claimedAt,
		msg:       message.NewMessage(token, []byte("payload")),
	}
	q.pending[token] = pc
}
