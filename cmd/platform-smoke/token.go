package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func mintToken(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	body := []byte(`{"subject":"1","login":"platform-smoke"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/auth/test-mint", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("test-mint request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("test-mint status %d: %s", resp.StatusCode, truncate(raw))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("test-mint decode: %w body=%s", err, truncate(raw))
	}
	return out.Token, nil
}
