package sqs

import (
	"testing"
)

func body_configTest_85(t *testing.T, tt struct {
	name    string
	mutate  func(cfg Config) Config
	wantErr bool
}) {
	err := tt.mutate(validConfig()).Validate()
	if (err != nil) != tt.wantErr {
		t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
	}
}
