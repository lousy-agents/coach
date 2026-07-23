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
	var missing []string

	workerID := os.Getenv("COACH_WORKER_ID")
	if workerID == "" {
		missing = append(missing, "COACH_WORKER_ID")
	}
	redisAddr := os.Getenv("COACH_REDIS_ADDR")
	if redisAddr == "" {
		missing = append(missing, "COACH_REDIS_ADDR")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("coach-worker: missing required env var(s): %s", strings.Join(missing, ", "))
	}

	cfg := Config{
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

	if raw := os.Getenv("COACH_REDIS_DB"); raw != "" {
		var db int
		if _, err := fmt.Sscanf(raw, "%d", &db); err != nil {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_REDIS_DB %q: %w", raw, err)
		}
		cfg.RedisDB = db
	}
	for _, pair := range []struct {
		env string
		dst *time.Duration
	}{
		{"COACH_WORKER_HEARTBEAT_INTERVAL", &cfg.HeartbeatInterval},
		{"COACH_WORKER_STALE_AFTER", &cfg.StaleAfter},
		{"COACH_WORKER_RECONCILE_INTERVAL", &cfg.ReconcileInterval},
		{"COACH_WORKER_QUEUED_AGE_THRESHOLD", &cfg.QueuedAgeThreshold},
		{"COACH_WORKER_IDLE_POLL_INTERVAL", &cfg.IdlePollInterval},
		{"COACH_REDIS_CLAIM_AFTER", &cfg.RedisClaimAfter},
	} {
		if raw := os.Getenv(pair.env); raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return Config{}, fmt.Errorf("coach-worker: invalid %s %q: %w", pair.env, raw, err)
			}
			*pair.dst = d
		}
	}

	if raw := os.Getenv("COACH_WORKER_MAX_ATTEMPTS"); raw != "" {
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_WORKER_MAX_ATTEMPTS %q (must be integer >= 1)", raw)
		}
		cfg.MaxAttempts = n
	}

	if cfg.StaleAfter < 3*cfg.HeartbeatInterval {
		return Config{}, fmt.Errorf(
			"coach-worker: COACH_WORKER_STALE_AFTER (%s) must be >= 3× COACH_WORKER_HEARTBEAT_INTERVAL (%s)",
			cfg.StaleAfter, cfg.HeartbeatInterval,
		)
	}

	return cfg, nil
}

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
