package authn_test

import (
	"encoding/json"
	"net/http"
)

type sigbodyoauthAcceptanceTestexchangesTheCodeOnBaseURLFetchesUs struct {
	ghAccessToken string
	ghUserID      int64
	ghLogin       string
	userHits      *int
}

func (sigRecv *sigbodyoauthAcceptanceTestexchangesTheCodeOnBaseURLFetchesUs) call(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/user" {
		http.Error(w, "not found on api host: "+r.URL.Path, http.StatusNotFound)
		return
	}
	*sigRecv.userHits++
	if r.Header.Get("Authorization") != "Bearer "+sigRecv.ghAccessToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    sigRecv.ghUserID,
		"login": sigRecv.ghLogin,
	})
}
