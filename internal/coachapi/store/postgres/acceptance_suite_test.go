package postgres_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestPostgresStoreAcceptance runs the Ginkgo acceptance suite for the
// Postgres coachapi.WorkerJobStore (Task 2 / GitHub issue #103). Specs skip
// unless COACH_PG_DSN points at a reachable Postgres 16+ instance.
func TestPostgresStoreAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/store/postgres acceptance suite")
}
