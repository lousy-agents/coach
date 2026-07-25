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

	When("baseline budget env vars are set to 0", func() {
		It("accepts 0 as unlimited for MaxFiles and MaxTotalBytes", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_BASELINE_MAX_FILES", "0")
			setenv("COACH_BASELINE_MAX_TOTAL_BYTES", "0")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.BaselineMaxFiles).To(Equal(0))
			Expect(cfg.BaselineMaxTotalBytes).To(Equal(int64(0)))
		})
	})

	When("baseline budget env vars are negative", func() {
		It("rejects negative MaxFiles", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_BASELINE_MAX_FILES", "-1")

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_BASELINE_MAX_FILES"))
		})
	})

	When("judgment / packing env vars are unset", func() {
		It("loads local-LLM defaults for wall time, judgment cap, and pack knobs", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.JudgmentMaxWallTime).To(Equal(10 * time.Minute))
			// 0 is the handler "use default 16" sentinel (negative = unlimited).
			Expect(cfg.MaxHiddenMutationJudgments).To(Equal(0))
			Expect(cfg.MaxFindingsPerJudgmentPack).To(Equal(4))
			Expect(cfg.MaxJudgmentPromptTokens).To(Equal(3500))
			Expect(cfg.JudgmentFileAffinityMinFindings).To(Equal(5))
			Expect(cfg.JudgmentEvidenceWindowLines).To(Equal(15))
		})
	})

	When("judgment / packing env vars are set", func() {
		It("parses wall time, judgment cap, and pack knobs into Config", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_JUDGMENT_MAX_WALL_TIME", "12m")
			setenv("COACH_MAX_HIDDEN_MUTATION_JUDGMENTS", "8")
			setenv("COACH_MAX_FINDINGS_PER_JUDGMENT_PACK", "2")
			setenv("COACH_MAX_JUDGMENT_PROMPT_TOKENS", "2000")
			setenv("COACH_JUDGMENT_FILE_AFFINITY_MIN_FINDINGS", "3")
			setenv("COACH_JUDGMENT_EVIDENCE_WINDOW_LINES", "10")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.JudgmentMaxWallTime).To(Equal(12 * time.Minute))
			Expect(cfg.MaxHiddenMutationJudgments).To(Equal(8))
			Expect(cfg.MaxFindingsPerJudgmentPack).To(Equal(2))
			Expect(cfg.MaxJudgmentPromptTokens).To(Equal(2000))
			Expect(cfg.JudgmentFileAffinityMinFindings).To(Equal(3))
			Expect(cfg.JudgmentEvidenceWindowLines).To(Equal(10))
		})

		It("accepts negative COACH_MAX_HIDDEN_MUTATION_JUDGMENTS as unlimited", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_MAX_HIDDEN_MUTATION_JUDGMENTS", "-1")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.MaxHiddenMutationJudgments).To(Equal(-1))
		})

		It("treats COACH_MAX_HIDDEN_MUTATION_JUDGMENTS=0 as the default-cap sentinel", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_MAX_HIDDEN_MUTATION_JUDGMENTS", "0")

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.MaxHiddenMutationJudgments).To(Equal(0))
		})
	})

	When("COACH_JUDGMENT_MAX_WALL_TIME is below the 5m minimum", func() {
		It("fails fast rather than allowing an unsafe judgment wall", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_JUDGMENT_MAX_WALL_TIME", "4m")

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_JUDGMENT_MAX_WALL_TIME"))
		})
	})

	When("pack knob env vars are non-positive", func() {
		It("rejects COACH_MAX_FINDINGS_PER_JUDGMENT_PACK below 1", func() {
			setenv("COACH_WORKER_ID", "w1")
			setenv("COACH_REDIS_ADDR", "127.0.0.1:6379")
			setenv("COACH_MAX_FINDINGS_PER_JUDGMENT_PACK", "0")

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_MAX_FINDINGS_PER_JUDGMENT_PACK"))
		})
	})
})
