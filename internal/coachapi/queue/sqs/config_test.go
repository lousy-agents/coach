package sqs

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type staticCreds struct{}

func (staticCreds) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: "fake", SecretAccessKey: "fake"}, nil
}

func validConfig() Config {
	return Config{
		Region:            "us-east-1",
		QueueURL:          "https://sqs.us-east-1.amazonaws.com/123456789012/coach-jobs",
		VisibilityTimeout: time.Minute,
		Credentials:       staticCreds{},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(cfg Config) Config
		wantErr bool
	}{
		{
			name:    "valid config",
			mutate:  func(cfg Config) Config { return cfg },
			wantErr: false,
		},
		{
			name:    "missing region",
			mutate:  func(cfg Config) Config { cfg.Region = ""; return cfg },
			wantErr: true,
		},
		{
			name:    "missing queue URL",
			mutate:  func(cfg Config) Config { cfg.QueueURL = ""; return cfg },
			wantErr: true,
		},
		{
			name:    "zero visibility timeout",
			mutate:  func(cfg Config) Config { cfg.VisibilityTimeout = 0; return cfg },
			wantErr: true,
		},
		{
			name:    "negative visibility timeout",
			mutate:  func(cfg Config) Config { cfg.VisibilityTimeout = -time.Second; return cfg },
			wantErr: true,
		},
		{
			name:    "missing credentials",
			mutate:  func(cfg Config) Config { cfg.Credentials = nil; return cfg },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(validConfig()).Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPoisonQueueName(t *testing.T) {
	if got, want := poisonQueueName("coach-jobs"), "coach-jobs-poison"; got != want {
		t.Errorf("poisonQueueName(%q) = %q, want %q", "coach-jobs", got, want)
	}
}

func TestQueueNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://sqs.us-east-1.amazonaws.com/123456789012/coach-jobs", "coach-jobs"},
		{"https://sqs.us-east-1.amazonaws.com/123456789012/coach-jobs/", "coach-jobs"},
		{"coach-jobs", "coach-jobs"},
	}
	for _, tt := range tests {
		if got := queueNameFromURL(tt.url); got != tt.want {
			t.Errorf("queueNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
