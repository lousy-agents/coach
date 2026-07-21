// Command thinproof-runner is the external test-runner container for issue
// #79's Task 0.3 thin offline proof: it exercises the fake-GitHub ->
// pkg/githubingest -> pkg/semantics -> pkg/codesignal path against a fake
// GitHub service reachable only at FAKE_GITHUB_BASE_URL (e.g.
// http://fake-github:8080 inside Compose), through a GuardedTransport that
// allows egress to that host only, and writes its findings to OUTPUT_PATH
// as JSON.
//
// It deliberately does not assert pass/fail against a golden itself: that
// comparison happens host-side, in
// internal/acceptanceharness/thinproof/compose_acceptance_test.go, so the
// golden stays a committed, reviewable test fixture rather than a value
// baked into this binary.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/thinproof"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// resultSchemaVersion versions thinproofResult's on-disk shape, independent
// of acceptanceharness.FixtureSchemaVersion and codesignal.Report's own
// SchemaVersion, per docs/architecture/acceptance-harness.md section 3's
// golden-fixture-versioning rules.
const resultSchemaVersion = 1

// thinproofResult is the JSON shape written to OUTPUT_PATH, decoded by
// compose_acceptance_test.go on the host side.
type thinproofResult struct {
	SchemaVersion     int                                     `json:"schema_version"`
	Report            *codesignal.Report                      `json:"report"`
	FileMetadata      githubingest.FileMetadata               `json:"file_metadata"`
	GuardResult       acceptanceharness.CredentialGuardResult `json:"guard_result"`
	BlockedRequests   []string                                `json:"blocked_requests"`
	FakeGitHubRecords []acceptanceharness.RequestRecord       `json:"fake_github_records"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "thinproof-runner: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("thinproof-runner: proof succeeded")
}

func run() error {
	baseURL := os.Getenv("FAKE_GITHUB_BASE_URL")
	if baseURL == "" {
		return fmt.Errorf("step 1 (read FAKE_GITHUB_BASE_URL): environment variable is required and unset")
	}

	outputPath := os.Getenv("OUTPUT_PATH")
	if outputPath == "" {
		outputPath = "/output/result.json"
	}

	guardResult := acceptanceharness.ScanProcessEnv()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("step 4 (parse FAKE_GITHUB_BASE_URL): %w", err)
	}
	allowedHost := parsed.Host

	if err := waitForHost(allowedHost, 30*time.Second); err != nil {
		return fmt.Errorf("step 4 (wait for fake-github at %s to accept connections): %w", allowedHost, err)
	}

	transport := acceptanceharness.NewGuardedTransport([]string{allowedHost}, http.DefaultTransport)
	client := &http.Client{Transport: transport}

	privateKeyPEM, err := generateRSAPrivateKeyPEM()
	if err != nil {
		return fmt.Errorf("step 5 (generate RSA private key): %w", err)
	}

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          thinproof.AppID,
		InstallationID: thinproof.InstallationID,
		PrivateKey:     privateKeyPEM,
		BaseURL:        baseURL,
		Transport:      transport,
	})
	if err != nil {
		return fmt.Errorf("step 6 (build GitHubFileReader): %w", err)
	}

	ctx := context.Background()
	content, meta, err := reader.ReadFile(ctx, githubingest.GitHubFileRef{
		Owner: thinproof.Owner,
		Repo:  thinproof.Repo,
		Ref:   thinproof.Ref,
		Path:  thinproof.Path,
	})
	if err != nil {
		return fmt.Errorf("step 7 (ReadFile against fake GitHub): %w", err)
	}

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return fmt.Errorf("step 8 (NewAnalyzer): %w", err)
	}
	result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
		Path:     thinproof.Path,
		Language: semantics.LanguageGo,
		Content:  content,
	})
	if err != nil {
		return fmt.Errorf("step 8 (AnalyzeBytes): %w", err)
	}

	builder, err := codesignal.New(codesignal.Options{})
	if err != nil {
		return fmt.Errorf("step 9 (codesignal.New): %w", err)
	}
	report, err := builder.Build(ctx, codesignal.Input{
		Files: []codesignal.FileChange{
			{Path: thinproof.Path, Status: "modified", Head: result},
		},
	})
	if err != nil {
		return fmt.Errorf("step 9 (codesignal Build): %w", err)
	}

	records, err := fetchFakeGitHubRecords(ctx, client, baseURL)
	if err != nil {
		return fmt.Errorf("step 10 (fetch /__test__/records): %w", err)
	}

	out := thinproofResult{
		SchemaVersion:     resultSchemaVersion,
		Report:            report,
		FileMetadata:      meta,
		GuardResult:       guardResult,
		BlockedRequests:   transport.BlockedRequests(),
		FakeGitHubRecords: records,
	}

	if err := writeResult(outputPath, out); err != nil {
		return fmt.Errorf("step 11 (write OUTPUT_PATH): %w", err)
	}

	return nil
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
