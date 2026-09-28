package sqs_test

import "encoding/json"

type sigcallS3 struct {
	body   []byte
	health *struct {
		Services map[string]string "json:\"services\""
	}
	readErr error
}

func (sigRecv *sigcallS3) call() (bool, bool, bool) {

	if sigRecv.readErr == nil && json.Unmarshal(sigRecv.body, sigRecv.health) == nil {
		if status := (*sigRecv.health).Services["sqs"]; status == "available" || status == "running" {
			return true, true, true
		}
	}
	return false, false, false
}
