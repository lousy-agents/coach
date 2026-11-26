package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	coachEvalBinary string
	buildDir        string
)

func TestCoachEvalAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "coach-eval acceptance suite")
}

var _ = BeforeSuite(func() {
	var err error
	buildDir, err = os.MkdirTemp("", "coach-eval-acceptance")
	Expect(err).NotTo(HaveOccurred())

	coachEvalBinary = filepath.Join(buildDir, "coach-eval")
	build := exec.Command("go", "build", "-o", coachEvalBinary, "./cmd/coach-eval")
	build.Dir = filepath.Join("..", "..")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(buildDir)).To(Succeed())
})
