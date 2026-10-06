package sqs_test

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// createLocalStackQueue creates a fresh SQS queue named queueName against
// LocalStack via a plain HTTP call (not the sqs package under test, so this
// setup step exercises an independent path from what's under test) and
// returns its queue URL.
func createLocalStackQueue(t *testing.T, endpoint, queueName string) (string, bool) {
	t.Helper()

	form := "Action=CreateQueue&QueueName=" + queueName + "&Version=2012-11-05"
	req, err := http.NewRequest(http.MethodPost, endpoint+"/", strings.NewReader(form))
	if err != nil {
		t.Skipf("building CreateQueue request failed; skipping: %v", err)
		return "", false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// LocalStack's SQS query-protocol endpoint accepts unsigned requests
	// with any Authorization header shape for local testing.
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=localstack-fake-access-key/20260101/us-east-1/sqs/aws4_request")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("CreateQueue against LocalStack failed; skipping: %v", err)
		return "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Skipf("CreateQueue against LocalStack returned %d; skipping: %s", resp.StatusCode, body)
		return "", false
	}

	queueURL := extractQueueURL(string(body))
	if queueURL == "" {
		t.Skipf("could not parse a queue URL from CreateQueue response; skipping: %s", body)
		return "", false
	}
	return queueURL, true
}

func deleteLocalStackQueue(endpoint, queueURL string) {
	if queueURL == "" {
		return
	}
	form := "Action=DeleteQueue&QueueUrl=" + queueURL + "&Version=2012-11-05"
	req, err := http.NewRequest(http.MethodPost, endpoint+"/", strings.NewReader(form))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generating random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}

// extractQueueURL pulls <QueueUrl>...</QueueUrl> out of an SQS
// CreateQueueResponse XML body.
func extractQueueURL(body string) string {
	const open, close = "<QueueUrl>", "</QueueUrl>"
	start := strings.Index(body, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(body[start:], close)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}
