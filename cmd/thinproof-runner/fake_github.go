package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// waitForHost retries a plain TCP dial against hostport until it succeeds
// or timeout elapses, so a Compose ordering race (runner starting before
// fake-github is accepting connections) fails with a clear timeout instead
// of a flaky first-attempt connection-refused error.
func waitForHost(hostport string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", hostport, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s waiting for %s: %w", timeout, hostport, lastErr)
}

func fetchFakeGitHubRecords(ctx context.Context, client *http.Client, baseURL string) ([]acceptanceharness.RequestRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/__test__/records", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var records []acceptanceharness.RequestRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, err
	}
	return records, nil
}
