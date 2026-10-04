package coachapi_test

import (
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func newTestServer(store coachapi.JobStore, az authz.RepoAuthorizer, q queue.TaskQueue, now func() time.Time, newJobID func() string) *coachapi.Server {
	srv, err := coachapi.NewServer(coachapi.ServerConfig{
		Store:      store,
		Authorizer: az,
		Queue:      q,
		Now:        now,
		NewJobID:   newJobID,
	})
	Expect(err).NotTo(HaveOccurred())
	return srv
}
