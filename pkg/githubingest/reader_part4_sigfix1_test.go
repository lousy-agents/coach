package githubingest_test

import "net/http"

type sigTestReadFileDirectoryListingFailureMapsToSentinel43067476 struct {
	fileContents string
	tt           struct {
		name string

		status int

		want error
	}
}

func (sigRecv *sigTestReadFileDirectoryListingFailureMapsToSentinel43067476) call(req *http.Request) *http.Response {
	return body_readerPart4Test_138(req, sigRecv.fileContents, sigRecv.tt)
}
