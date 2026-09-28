package sqs_test

import (
	"strings"
)

// parseHostPort extracts the host port from `docker port <id> 4566/tcp`
// output, which looks like "0.0.0.0:32768\n[::]:32768\n".
func parseHostPort(portOutput string) string {
	for _, line := range strings.Split(strings.TrimSpace(portOutput), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.LastIndex(line, ":")
		if idx < 0 || idx == len(line)-1 {
			continue
		}
		return line[idx+1:]
	}
	return ""
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
