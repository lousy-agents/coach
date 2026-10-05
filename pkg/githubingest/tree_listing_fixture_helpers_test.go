package githubingest_test

import (
	"net/http"
	"strings"
)

func serveTreeListingFixture(req *http.Request, byDir map[string]string) *http.Response {
	for dir, body := range byDir {
		var suffix string
		if dir == "" {
			suffix = "/contents/?"
		} else {
			suffix = "/contents/" + dir + "?"
		}
		if strings.Contains(req.URL.String(), suffix) {
			return jsonResponse(req, http.StatusOK, body)
		}
	}
	panic("treeContentsRouter: no fixture registered for request " + req.URL.String())
}
