package authn_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthnAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/authn acceptance suite")
}
