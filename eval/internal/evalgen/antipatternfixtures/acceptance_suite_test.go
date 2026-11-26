package antipatternfixtures_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAntipatternFixturesAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "profile-derived anti-pattern fixtures acceptance suite")
}
