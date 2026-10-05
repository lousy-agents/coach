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

func body_oauthAcceptanceTest_returns400InvalidRequest_130(mutateQuery func(code, goodState string) url.Values, setupClock func(now *time.Time, base time.Time)) {
	_, gh := newOAuthFake()
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	now := base
	svc := newOAuthService(gh.URL(), func() time.Time { return now }, time.Minute)
	h := svc.Handler()
	client := noRedirectClient()
	coach := httptest.NewServer(h)
	DeferCleanup(coach.Close)

	goodState := startOAuthAndParseState(client, coach.URL)

	authResp, err := client.Get(gh.URL() + "/login/oauth/authorize?" + url.Values{
		"client_id":     {oauthClientID},
		"redirect_uri":  {oauthRedirectURI},
		"state":         {goodState},
		"scenario_code": {oauthScenarioCode},
	}.Encode())
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = authResp.Body.Close() })
	Expect(authResp.StatusCode).To(Equal(http.StatusFound))
	u, err := url.Parse(authResp.Header.Get("Location"))
	Expect(err).NotTo(HaveOccurred())
	code := u.Query().Get("code")
	Expect(code).NotTo(BeEmpty())

	now = base
	if setupClock != nil {
		setupClock(&now, base)
	}
	query := mutateQuery(code, goodState)
	status, body := doReq(h, http.MethodGet, "/oauth/github/callback?"+query.Encode(), "", nil)
	expectInvalidRequest(status, body)
}

func body_oauthAcceptanceTest_exchangesTheCodeOnBaseURLFetchesUserOnAPIBaseURL_184() {
	const (
		ghAccessToken = "gho_split_host_token"
		ghUserID      = int64(99)
		ghLogin       = "api-host-user"
	)

	var tokenHits, userHits int

	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/login/oauth/authorize":
			http.Error(w, "authorize not used in this test", http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
			tokenHits++
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
				"access_token": ghAccessToken,
				"token_type":   "bearer",
				"scope":        "",
			})
		case r.URL.Path == "/user":
			Fail("GET /user hit OAuth BaseURL host; want APIBaseURL")
			http.Error(w, "wrong host for /user", http.StatusNotFound)
		default:
			http.Error(w, "not found on oauth host: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	DeferCleanup(oauthSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc((&sigbodyoauthAcceptanceTestexchangesTheCodeOnBaseURLFetchesUs{ghAccessToken: ghAccessToken, ghUserID: ghUserID, ghLogin: ghLogin, userHits: &userHits}).call))
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
	Expect(tokenHits).To(Equal(1))
	Expect(userHits).To(Equal(1))

	meCode, meBody := doReq(svc.Handler(), http.MethodGet, "/v1/me", tokResp.AccessToken, nil)
	Expect(meCode).To(Equal(http.StatusOK), "body=%s", meBody)
	var p coachapi.Principal
	Expect(json.Unmarshal(meBody, &p)).To(Succeed(), "body=%s", meBody)
	Expect(p).To(Equal(coachapi.Principal{
		Provider: "github",
		Subject:  strconv.FormatInt(ghUserID, 10),
		Login:    ghLogin,
	}))
}

func body_oauthAcceptanceTest_307(w http.ResponseWriter, r *http.Request, user map[string]any, ghAccessToken string) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": ghAccessToken,
			"token_type":   "bearer",
			"scope":        "",
		})
	case r.Method == http.MethodGet && r.URL.Path == "/user":
		if r.Header.Get("Authorization") != "Bearer "+ghAccessToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	default:
		http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
	}
}
