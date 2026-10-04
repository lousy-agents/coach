package githubingest_test

import (
	"net/http"
	"strings"
)

func body_readerPart4Test_138(req *http.Request, fileContents string, tt struct {
	name   string
	status int
	want   error
}) *http.Response {
	if strings.HasSuffix(req.URL.Path, "/contents/dir/hello.txt") {
		return jsonResponse(req, http.StatusOK, fileContents)
	}
	return jsonResponse(req, tt.status, `{"message":"denied"}`)
}
