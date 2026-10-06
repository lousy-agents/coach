package authn_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

var _ = Describe("GitHub OAuth identity for Coach JWT minting", func() {
	When("APIBaseURL differs from BaseURL (real GitHub host split)", func() {
		It("exchanges the code on BaseURL, fetches /user on APIBaseURL, and mints a Coach JWT for that identity", func() {
			const (
				ghAccessToken = "gho_split_host_token"
				ghUserID      = int64(99)
				ghLogin       = "api-host-user"
			)

			oauthHost := &oauthHostTokenExchange{accessToken: ghAccessToken}
			oauthSrv := httptest.NewServer(oauthHost)
			DeferCleanup(oauthSrv.Close)

			apiHost := &apiHostUser{accessToken: ghAccessToken, userID: ghUserID, login: ghLogin}
			apiSrv := httptest.NewServer(apiHost)
			DeferCleanup(apiSrv.Close)

			svc, err := authn.New(authn.Options{
				SigningKey: []byte(testSecret),
				Issuer:     testIssuer,
				TokenTTL:   time.Hour,
				Now:        fixedNow(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)),
				Denylist:   authn.NewMemoryDenylist(),
				GitHubOAuth: &authn.GitHubOAuthConfig{
					ClientID:     oauthClientID,
					ClientSecret: oauthClientSecret,
					BaseURL:      oauthSrv.URL,
					APIBaseURL:   apiSrv.URL,
					RedirectURI:  oauthRedirectURI,
				},
				OAuthState:    authn.NewMemoryOAuthState(),
				OAuthStateTTL: 10 * time.Minute,
			})
			Expect(err).NotTo(HaveOccurred())

			client := noRedirectClient()
			coach := httptest.NewServer(svc.Handler())
			DeferCleanup(coach.Close)

			startResp, err := client.Get(coach.URL + "/oauth/github/start")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = startResp.Body.Close() })
			Expect(startResp.StatusCode).To(Equal(http.StatusFound))
			loc := startResp.Header.Get("Location")
			Expect(loc).To(HavePrefix(oauthSrv.URL + "/login/oauth/authorize"))
			authURL, err := url.Parse(loc)
			Expect(err).NotTo(HaveOccurred())
			state := authURL.Query().Get("state")
			Expect(state).NotTo(BeEmpty())

			cbResp, err := client.Get(coach.URL + "/oauth/github/callback?" + url.Values{
				"code":  {"split-host-code"},
				"state": {state},
			}.Encode())
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = cbResp.Body.Close() })
			cbBody, _ := io.ReadAll(cbResp.Body)
			Expect(cbResp.StatusCode).To(Equal(http.StatusOK), "body=%s", cbBody)
			var tokResp struct {
				AccessToken string `json:"access_token"`
				TokenType   string `json:"token_type"`
			}
			Expect(json.Unmarshal(cbBody, &tokResp)).To(Succeed(), "body=%s", cbBody)
			Expect(tokResp.AccessToken).NotTo(BeEmpty())
			Expect(oauthHost.tokenHits).To(Equal(1))
			Expect(apiHost.userHits).To(Equal(1))

			meCode, meBody := doReq(svc.Handler(), http.MethodGet, "/v1/me", tokResp.AccessToken, nil)
			Expect(meCode).To(Equal(http.StatusOK), "body=%s", meBody)
			var p coachapi.Principal
			Expect(json.Unmarshal(meBody, &p)).To(Succeed(), "body=%s", meBody)
			Expect(p).To(Equal(coachapi.Principal{
				Provider: "github",
				Subject:  strconv.FormatInt(ghUserID, 10),
				Login:    ghLogin,
			}))
		})
	})
})

// oauthHostTokenExchange is the BaseURL host of a split-host GitHub: it
// exchanges a code for accessToken, counting exchanges in tokenHits, and
// fails the spec if GET /user ever reaches it instead of the API host.
type oauthHostTokenExchange struct {
	accessToken string
	tokenHits   int
}

func (h *oauthHostTokenExchange) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/login/oauth/authorize":
		http.Error(w, "authorize not used in this test", http.StatusNotFound)
	case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
		h.tokenHits++
		Expect(r.ParseForm()).To(Succeed())
		if r.Form.Get("client_id") != oauthClientID || r.Form.Get("client_secret") != oauthClientSecret {
			http.Error(w, "bad client", http.StatusUnauthorized)
			return
		}
		if r.Form.Get("code") == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": h.accessToken,
			"token_type":   "bearer",
			"scope":        "",
		})
	case r.URL.Path == "/user":
		Fail("GET /user hit OAuth BaseURL host; want APIBaseURL")
		http.Error(w, "wrong host for /user", http.StatusNotFound)
	default:
		http.Error(w, "not found on oauth host: "+r.URL.Path, http.StatusNotFound)
	}
}

// apiHostUser is the APIBaseURL host of a split-host GitHub: it serves only
// GET /user for accessToken, counting requests in userHits.
type apiHostUser struct {
	accessToken string
	userID      int64
	login       string
	userHits    int
}

func (h *apiHostUser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/user" {
		http.Error(w, "not found on api host: "+r.URL.Path, http.StatusNotFound)
		return
	}
	h.userHits++
	if r.Header.Get("Authorization") != "Bearer "+h.accessToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    h.userID,
		"login": h.login,
	})
}
