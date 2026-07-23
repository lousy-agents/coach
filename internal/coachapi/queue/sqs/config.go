package sqs

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// defaultHTTPTimeout bounds every SQS API call this adapter makes so an
// unreachable endpoint (a wrong LocalStack address, a network partition to
// AWS) fails fast instead of hanging a Claim/Complete/Nack call forever, per
// AGENTS.md's outbound-HTTP policy.
const defaultHTTPTimeout = 10 * time.Second

// Config configures a Queue against one SQS queue, matching ADR-006's
// "Provider configuration" aws-sqs shape (region/queueURL/visibilityTimeout).
type Config struct {
	Region            string
	QueueURL          string
	VisibilityTimeout time.Duration

	// Endpoint overrides the SQS API endpoint (e.g. a LocalStack address).
	// Empty means the real AWS SQS endpoint for Region.
	Endpoint string

	// Credentials is required: this package never resolves ambient AWS
	// credentials on the caller's behalf (see the package doc comment's
	// "Credential safety" section), so callers must supply their own
	// aws.CredentialsProvider explicitly.
	Credentials aws.CredentialsProvider

	// PoisonQueueURL overrides the poison-task destination queue. Empty
	// means Queue derives and creates-if-missing "<queue name>-poison" at
	// construction time (see the package doc comment).
	PoisonQueueURL string

	// HTTPTimeout bounds every SQS API call. Zero means defaultHTTPTimeout.
	HTTPTimeout time.Duration
}

// Validate reports the first configuration error found, or nil if cfg is
// usable by NewQueue.
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Region) == "" {
		return fmt.Errorf("sqs: Config.Region is required")
	}
	if strings.TrimSpace(cfg.QueueURL) == "" {
		return fmt.Errorf("sqs: Config.QueueURL is required")
	}
	if cfg.VisibilityTimeout < time.Second {
		return fmt.Errorf("sqs: Config.VisibilityTimeout must be at least 1s (SQS's VisibilityTimeout is a whole number of seconds), got %s", cfg.VisibilityTimeout)
	}
	if cfg.VisibilityTimeout%time.Second != 0 {
		return fmt.Errorf("sqs: Config.VisibilityTimeout must be a whole number of seconds (SQS truncates sub-second precision), got %s", cfg.VisibilityTimeout)
	}
	if cfg.Credentials == nil {
		return fmt.Errorf("sqs: Config.Credentials is required (this package never resolves ambient AWS credentials)")
	}
	return nil
}

func (cfg Config) httpTimeout() time.Duration {
	if cfg.HTTPTimeout > 0 {
		return cfg.HTTPTimeout
	}
	return defaultHTTPTimeout
}

// poisonQueueName derives the default poison-task destination queue name
// from a main queue's name (the last path segment of its URL): "coach-jobs"
// becomes "coach-jobs-poison".
func poisonQueueName(mainQueueName string) string {
	return mainQueueName + "-poison"
}

// queueNameFromURL extracts the queue name (the last "/"-separated
// segment) from an SQS queue URL, e.g.
// "https://sqs.us-east-1.amazonaws.com/123456789/coach-jobs" -> "coach-jobs".
func queueNameFromURL(queueURL string) string {
	trimmed := strings.TrimRight(queueURL, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return trimmed
	}
	return trimmed[idx+1:]
}
