package codesignalcli

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProjectTextAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "project text renderer acceptance suite")
}
