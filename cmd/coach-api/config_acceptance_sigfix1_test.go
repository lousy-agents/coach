package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sigclearEnvS155930721 struct {
	key string
}

func (sigRecv *sigclearEnvS155930721) call() {

	if v, ok := os.LookupEnv(sigRecv.key); ok {
		DeferCleanup(func() { Expect(os.Setenv(sigRecv.key, v)).To(Succeed()) })
	}
}
