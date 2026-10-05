package githubingest_test

import (
	"net/http"

	"strings"
)

func body_readerPart4Test_97(req *http.Request, resolvedFileContents string, dirListingWithSymlinkEntry string) *http.Response {
	if strings.HasSuffix(req.URL.Path, "/contents/dir/link.txt") {
		return jsonResponse(req, http.StatusOK, resolvedFileContents)
	}
	return jsonResponse(req, http.StatusOK, dirListingWithSymlinkEntry)
}
