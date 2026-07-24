package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultRedisStream        = "coach-jobs"
	defaultRedisConsumerGroup = "coach-workers"
	defaultRedisClaimAfter    = 5 * time.Minute
	defaultHeartbeatInterval  = 15 * time.Second
	defaultStaleAfter         = 60 * time.Second
	defaultReconcileInterval  = 30 * time.Second
	defaultQueuedAgeThreshold = 30 * time.Second
	defaultIdlePollInterval   = time.Second
	defaultMaxAttempts        = 5
)

// Config holds cmd/coach-worker environment-driven settings.
type Config struct {
	WorkerID string

	HeartbeatInterval  time.Duration
	StaleAfter         time.Duration
	ReconcileInterval  time.Duration
	QueuedAgeThreshold time.Duration
	IdlePollInterval   time.Duration
	MaxAttempts        int

	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	RedisStream        string
	RedisConsumerGroup string
	RedisConsumer      string
	RedisClaimAfter    time.Duration

	// PostgresDSN selects PostgresStore when set; MemoryStore when empty
	// (local/dev only — production must set COACH_PG_DSN).
	PostgresDSN string
}

func loadConfigFromEnv() (Config, error) {
	workerID, redisAddr, err := requiredEnv()
	if err != nil {
		return Config{}, err
	}
	cfg := defaultConfig(workerID, redisAddr)
	if err := applyOptionalEnv(&cfg); err != nil {
		return Config{}, err
	}
	return validateConfig(cfg)
}

func defaultConfig(workerID, redisAddr string) Config {
	return Config{
		WorkerID:           workerID,
		HeartbeatInterval:  defaultHeartbeatInterval,
		StaleAfter:         defaultStaleAfter,
		ReconcileInterval:  defaultReconcileInterval,
		QueuedAgeThreshold: defaultQueuedAgeThreshold,
		IdlePollInterval:   defaultIdlePollInterval,
		MaxAttempts:        defaultMaxAttempts,
		RedisAddr:          redisAddr,
		RedisPassword:      os.Getenv("COACH_REDIS_PASSWORD"),
		RedisStream:        valueOrDefault(os.Getenv("COACH_REDIS_STREAM"), defaultRedisStream),
		RedisConsumerGroup: valueOrDefault(os.Getenv("COACH_REDIS_CONSUMER_GROUP"), defaultRedisConsumerGroup),
		RedisConsumer:      valueOrDefault(os.Getenv("COACH_REDIS_CONSUMER"), workerID),
		RedisClaimAfter:    defaultRedisClaimAfter,
		PostgresDSN:        os.Getenv("COACH_PG_DSN"),
	}
}

func validateConfig(cfg Config) (Config, error) {
	if cfg.StaleAfter < 3*cfg.HeartbeatInterval {
		return Config{}, fmt.Errorf(
			"coach-worker: COACH_WORKER_STALE_AFTER (%s) must be >= 3× COACH_WORKER_HEARTBEAT_INTERVAL (%s)",
			cfg.StaleAfter, cfg.HeartbeatInterval,
		)
	}
	return cfg, nil
}

func requiredEnv() (workerID, redisAddr string, err error) {
	var missing []string
	workerID = os.Getenv("COACH_WORKER_ID")
	if workerID == "" {
		missing = append(missing, "COACH_WORKER_ID")
	}
	redisAddr = os.Getenv("COACH_REDIS_ADDR")
	if redisAddr == "" {
		missing = append(missing, "COACH_REDIS_ADDR")
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("coach-worker: missing required env var(s): %s", strings.Join(missing, ", "))
	}
	return workerID, redisAddr, nil
}

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
