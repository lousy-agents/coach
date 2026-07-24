package main

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cmd/coach-worker config", func() {
	var envKeys []string

	setenv := func(k, v string) {
		envKeys = append(envKeys, k)
		Expect(os.Setenv(k, v)).To(Succeed())
	}

	BeforeEach(func() {
		envKeys = nil
	})

	AfterEach(func() {
		for _, k := range envKeys {
			_ = os.Unsetenv(k)
		}
	})

	When("required env vars are present", func() {
		It("loads defaults for heartbeat (15s), stale (60s), and max attempts (5)", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.WorkerID).To(Equal("w1"))
			Expect(cfg.HeartbeatInterval).To(Equal(15 * time.Second))
			Expect(cfg.StaleAfter).To(Equal(60 * time.Second))
			Expect(cfg.MaxAttempts).To(Equal(5))
			Expect(cfg.RedisStream).To(Equal("coach-jobs"))
			Expect(cfg.RedisConsumerGroup).To(Equal("coach-workers"))
			Expect(cfg.RedisConsumer).To(Equal("w1"))
		})
	})

	When("COACH_WORKER_MAX_ATTEMPTS is set below 1", func() {
		It("fails fast rather than allowing unbounded or zero attempts", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_WORKER_MAX_ATTEMPTS", "0")

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_WORKER_MAX_ATTEMPTS"))
		})
	})

	When("stale threshold is less than 3× heartbeat interval", func() {
		It("fails fast rather than starting with an unsafe reclaim window", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_WORKER_HEARTBEAT_INTERVAL", "15s")
			setenv("COACH_WORKER_STALE_AFTER", "30s")

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("3×"))
		})
	})

	When("required env vars are missing", func() {
		It("names every missing required var", func() {
			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_WORKER_ID"))
			Expect(err.Error()).To(ContainSubstring("COACH_REDIS_ADDR"))
		})
	})
})
