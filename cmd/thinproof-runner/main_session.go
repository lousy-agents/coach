package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/thinproof"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

type thinproofSession struct {
	baseURL     string
	outputPath  string
	guardResult acceptanceharness.CredentialGuardResult
	transport   *acceptanceharness.GuardedTransport
	client      *http.Client
	reader      *githubingest.GitHubFileReader
}

func openThinproofSession() (thinproofSession, error) {
	baseURL := os.Getenv("FAKE_GITHUB_BASE_URL")
	if baseURL == "" {
		return thinproofSession{}, fmt.Errorf("step 1 (read FAKE_GITHUB_BASE_URL): environment variable is required and unset")
	}

	outputPath := os.Getenv("OUTPUT_PATH")
	if outputPath == "" {
		outputPath = "/output/result.json"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return thinproofSession{}, fmt.Errorf("step 4 (parse FAKE_GITHUB_BASE_URL): %w", err)
	}
	allowedHost := parsed.Host

	if err := waitForHost(allowedHost, 30*time.Second); err != nil {
		return thinproofSession{}, fmt.Errorf("step 4 (wait for fake-github at %s to accept connections): %w", allowedHost, err)
	}

	transport := acceptanceharness.NewGuardedTransport([]string{allowedHost}, http.DefaultTransport)
	privateKeyPEM, err := generateRSAPrivateKeyPEM()
	if err != nil {
		return thinproofSession{}, fmt.Errorf("step 5 (generate RSA private key): %w", err)
	}

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          thinproof.AppID,
		InstallationID: thinproof.InstallationID,
		PrivateKey:     privateKeyPEM,
		BaseURL:        baseURL,
		Transport:      transport,
	})
	if err != nil {
		return thinproofSession{}, fmt.Errorf("step 6 (build GitHubFileReader): %w", err)
	}

	return thinproofSession{
		baseURL:     baseURL,
		outputPath:  outputPath,
		guardResult: acceptanceharness.ScanProcessEnv(),
		transport:   transport,
		client:      &http.Client{Transport: transport},
		reader:      reader,
	}, nil
}
