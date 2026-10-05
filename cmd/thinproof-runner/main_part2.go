package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"net"
	"net/http"

	"os"
	"path/filepath"
	"time"
)

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
func writeResult(path string, result thinproofResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "thinproof-runner: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("thinproof-runner: proof succeeded")
}

// generateRSAPrivateKeyPEM mirrors
// internal/acceptanceharness/testcreds.go's GenerateRSAPrivateKeyPEM, which
// takes a testing.TB and so cannot be called from this non-test binary.
func generateRSAPrivateKeyPEM() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block), nil
}
