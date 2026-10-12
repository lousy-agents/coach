package httpapi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestHTTPAPIAcceptance runs the Ginkgo acceptance suite for the /v1/jobs
// HTTP surface, driven through authn middleware and httpapi.Server.Handler.
func TestHTTPAPIAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/httpapi acceptance suite")
}
