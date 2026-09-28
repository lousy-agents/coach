package sqs_test

import (
	"io"
	"net/http"
)

type sigwaitForLocalStackReadyS1 struct {
	err  error
	resp *http.
		Response
}

func (sigRecv *sigwaitForLocalStackReadyS1) call() (bool, bool) {

	if sigRecv.err == nil {
		var health struct {
			Services map[string]string `json:"services"`
		}
		body, readErr := io.ReadAll(sigRecv.resp.Body)
		sigRecv.resp.
			Body.Close()
		if sigR0, sigR1, sigRet := (&sigcallS3{body: body, health: &health, readErr: readErr}).call(); sigRet {
			return sigR0, sigR1
		}

	}
	return false, false
}
