package projectmodel_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProjectmodelAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "projectmodel acceptance suite")
}
