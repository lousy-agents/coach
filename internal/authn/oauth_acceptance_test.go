package authn_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

const (
	oauthClientID     = "coach-oauth-client-id"
	oauthClientSecret = "coach-oauth-client-secret"
	oauthRedirectURI  = "http://coach.test/oauth/github/callback"
	oauthScenarioCode = "code-ok"
)

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func newOAuthFake() (*fakegithub.Fixture, *fakegithub.Server) {
	fx := fakegithub.NewFixture("authn-oauth")
	fx.OAuth.ClientID = oauthClientID
	fx.OAuth.ClientSecret = oauthClientSecret
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 42, Login: "octocat"}
	fx.OAuth.Codes[oauthScenarioCode] = fakegithub.OAuthCodeEntry{
		IdentityLogin: "octocat",
		Scenario:      fakegithub.ScenarioOK,
	}
	srv := fakegithub.NewServer(&fx)
	DeferCleanup(srv.Close)
	return &fx, srv
}

func newOAuthService(githubBase string, now func() time.Time, stateTTL time.Duration) *authn.Service {
	if now == nil {
		now = fixedNow(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	}
	if stateTTL <= 0 {
		stateTTL = 10 * time.Minute
	}
	svc, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     testIssuer,
		TokenTTL:   time.Hour,
		Now:        now,
		Denylist:   authn.NewMemoryDenylist(),
		GitHubOAuth: &authn.GitHubOAuthConfig{
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			BaseURL:      githubBase,
			RedirectURI:  oauthRedirectURI,
		},
		OAuthState:    authn.NewMemoryOAuthState(),
		OAuthStateTTL: stateTTL,
	})
	Expect(err).NotTo(HaveOccurred())
	return svc
}

func startOAuthAndParseState(client *http.Client, coachURL string) string {
	startResp, err := client.Get(coachURL + "/oauth/github/start")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = startResp.Body.Close() })
	Expect(startResp.StatusCode).To(Equal(http.StatusFound))
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	Expect(err).NotTo(HaveOccurred())
	state := authURL.Query().Get("state")
	Expect(state).NotTo(BeEmpty())
	return state
}

func expectInvalidRequest(code int, body []byte) {
	Expect(code).To(Equal(http.StatusBadRequest), "body=%s", body)
	env := decodeEnvelope(body)
	Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInvalidRequest))
}

func expectNoAccessToken(body []byte) {
	Expect(string(body)).NotTo(ContainSubstring("access_token"), "must not issue a token; body=%s", body)
}

var _ = Describe("GitHub OAuth identity for Coach JWT minting", func() {
	When("a user completes the fake-GitHub OAuth round-trip", func() {
		It("mints a Coach JWT that authorizes /v1/me, requests no scope on authorize, and rejects the GitHub OAuth access token on /v1", func() {
			_, gh := newOAuthFake()
			svc := newOAuthService(gh.URL(), nil, 0)
			coach := httptest.NewServer(svc.Handler())
			DeferCleanup(coach.Close)
			client := noRedirectClient()

			startResp, err := client.Get(coach.URL + "/oauth/github/start")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = startResp.Body.Close() })
			Expect(startResp.StatusCode).To(Equal(http.StatusFound), "body may help: read after status check")
			loc := startResp.Header.Get("Location")
			authURL, err := url.Parse(loc)
			Expect(err).NotTo(HaveOccurred(), "Location=%q", loc)
			Expect(loc).To(HavePrefix(gh.URL() + "/login/oauth/authorize"))
			q := authURL.Query()
			Expect(q.Get("client_id")).To(Equal(oauthClientID))
			Expect(q.Get("redirect_uri")).To(Equal(oauthRedirectURI))
			state := q.Get("state")
			Expect(state).NotTo(BeEmpty())
			if _, has := q["scope"]; has {
				Expect(strings.TrimSpace(q.Get("scope"))).To(BeEmpty(), "authorize must request no scope")
			}
			Expect(q.Get("scenario_code")).To(BeEmpty(), "start must not send scenario_code")

			fakeAuth := gh.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {oauthClientID},
				"redirect_uri":  {oauthRedirectURI},
				"state":         {state},
				"scenario_code": {oauthScenarioCode},
			}.Encode()
			authResp, err := client.Get(fakeAuth)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = authResp.Body.Close() })
			Expect(authResp.StatusCode).To(Equal(http.StatusFound))
			cbLoc, err := url.Parse(authResp.Header.Get("Location"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cbLoc.Query().Get("code")).NotTo(BeEmpty())
			Expect(cbLoc.Query().Get("state")).To(Equal(state))

			cbPath := "/oauth/github/callback?" + cbLoc.RawQuery
			cbResp, err := client.Get(coach.URL + cbPath)
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
			Expect(strings.EqualFold(tokResp.TokenType, "bearer")).To(BeTrue(), "token_type=%q", tokResp.TokenType)
			Expect(strings.Split(tokResp.AccessToken, ".")).To(HaveLen(3), "access_token must be Coach JWT")

			meCode, meBody := doReq(svc.Handler(), http.MethodGet, "/v1/me", tokResp.AccessToken, nil)
			Expect(meCode).To(Equal(http.StatusOK), "body=%s", meBody)
			var p coachapi.Principal
			Expect(json.Unmarshal(meBody, &p)).To(Succeed(), "body=%s", meBody)
			Expect(p).To(Equal(coachapi.Principal{
				Provider: "github",
				Subject:  strconv.FormatInt(42, 10),
				Login:    "octocat",
			}))

			fx := gh.Fixture()
			fx.OAuth.Codes["code-for-v1-reject"] = fakegithub.OAuthCodeEntry{
				IdentityLogin: "octocat",
				Scenario:      fakegithub.ScenarioOK,
			}
			exResp, err := http.PostForm(gh.URL()+"/login/oauth/access_token", url.Values{
				"client_id":     {oauthClientID},
				"client_secret": {oauthClientSecret},
				"code":          {"code-for-v1-reject"},
			})
			Expect(err).NotTo(HaveOccurred())
			var ghTok struct {
				AccessToken string `json:"access_token"`
			}
			Expect(json.NewDecoder(exResp.Body).Decode(&ghTok)).To(Succeed())
			Expect(exResp.Body.Close()).To(Succeed())
			Expect(ghTok.AccessToken).NotTo(BeEmpty())

			userReq, err := http.NewRequest(http.MethodGet, gh.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			userReq.Header.Set("Authorization", "Bearer "+ghTok.AccessToken)
			userResp, err := http.DefaultClient.Do(userReq)
			Expect(err).NotTo(HaveOccurred())
			userBody, _ := io.ReadAll(userResp.Body)
			Expect(userResp.Body.Close()).To(Succeed())
			Expect(userResp.StatusCode).To(Equal(http.StatusOK), "body=%s", userBody)

			rejCode, rejBody := doReq(svc.Handler(), http.MethodGet, "/v1/me", ghTok.AccessToken, nil)
			expectUnauthenticated(rejCode, rejBody)
		})
	})

	When("OAuth callback receives bad, missing, or expired state", func() {
		DescribeTable("returns 400 invalid_request",
			func(mutateQuery func(code, goodState string) url.Values, setupClock func(now *time.Time, base time.Time)) {
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
			},
			Entry("missing state",
				func(code, _ string) url.Values { return url.Values{"code": {code}} },
				nil,
			),
			Entry("unknown state",
				func(code, _ string) url.Values {
					return url.Values{"code": {code}, "state": {"never-issued-state"}}
				},
				nil,
			),
			Entry("expired state",
				func(code, goodState string) url.Values {
					return url.Values{"code": {code}, "state": {goodState}}
				},
				func(now *time.Time, base time.Time) { *now = base.Add(2 * time.Minute) },
			),
		)
	})

	When("APIBaseURL differs from BaseURL (real GitHub host split)", func() {
		It("exchanges the code on BaseURL, fetches /user on APIBaseURL, and mints a Coach JWT for that identity", func() {
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

			apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/user" {
					http.Error(w, "not found on api host: "+r.URL.Path, http.StatusNotFound)
					return
				}
				userHits++
				if r.Header.Get("Authorization") != "Bearer "+ghAccessToken {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":    ghUserID,
					"login": ghLogin,
				})
			}))
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
		})
	})

	When("GitHub /user returns an incomplete identity", func() {
		DescribeTable("returns 400 invalid_request and does not mint a Coach token",
			func(user map[string]any) {
				const ghAccessToken = "gho_incomplete_user"
				gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				}))
				DeferCleanup(gh.Close)

				svc := newOAuthService(gh.URL, nil, 0)
				client := noRedirectClient()
				coach := httptest.NewServer(svc.Handler())
				DeferCleanup(coach.Close)

				state := startOAuthAndParseState(client, coach.URL)
				status, body := doReq(svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
					"code":  {"incomplete-user-code"},
					"state": {state},
				}.Encode(), "", nil)
				expectInvalidRequest(status, body)
				expectNoAccessToken(body)
			},
			Entry("zero id with login", map[string]any{"id": 0, "login": "ghost"}),
			Entry("empty login with id", map[string]any{"id": int64(7), "login": ""}),
		)
	})

	When("OAuth CSRF state is reused after a successful callback", func() {
		It("rejects the second callback with 400 invalid_request even with a fresh code", func() {
			fx, gh := newOAuthFake()
			fx.OAuth.Codes["code-second"] = fakegithub.OAuthCodeEntry{
				IdentityLogin: "octocat",
				Scenario:      fakegithub.ScenarioOK,
			}
			svc := newOAuthService(gh.URL(), nil, 0)
			client := noRedirectClient()
			coach := httptest.NewServer(svc.Handler())
			DeferCleanup(coach.Close)

			state := startOAuthAndParseState(client, coach.URL)

			status, body := doReq(svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
				"code":  {oauthScenarioCode},
				"state": {state},
			}.Encode(), "", nil)
			Expect(status).To(Equal(http.StatusOK), "body=%s", body)
			var first struct {
				AccessToken string `json:"access_token"`
			}
			Expect(json.Unmarshal(body, &first)).To(Succeed())
			Expect(first.AccessToken).NotTo(BeEmpty())

			status, body = doReq(svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
				"code":  {"code-second"},
				"state": {state},
			}.Encode(), "", nil)
			expectInvalidRequest(status, body)
			expectNoAccessToken(body)
		})
	})

	When("GitHub redirects with error=access_denied", func() {
		It("returns 400 invalid_request and does not issue a Coach token", func() {
			_, gh := newOAuthFake()
			svc := newOAuthService(gh.URL(), nil, 0)
			h := svc.Handler()
			client := noRedirectClient()
			coach := httptest.NewServer(h)
			DeferCleanup(coach.Close)

			state := startOAuthAndParseState(client, coach.URL)
			q := url.Values{
				"error":             {"access_denied"},
				"error_description": {"The user has denied your application access."},
				"state":             {state},
			}
			status, body := doReq(h, http.MethodGet, "/oauth/github/callback?"+q.Encode(), "", nil)
			expectInvalidRequest(status, body)
			expectNoAccessToken(body)
		})
	})

	When("the OAuth state store errors", func() {
		It("fails closed with 503 internal_error on start Save", func() {
			storeErr := errors.New("oauth state store unavailable")
			base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
			svc, err := authn.New(authn.Options{
				SigningKey: []byte(testSecret),
				Issuer:     testIssuer,
				TokenTTL:   time.Hour,
				Now:        fixedNow(base),
				Denylist:   authn.NewMemoryDenylist(),
				GitHubOAuth: &authn.GitHubOAuthConfig{
					ClientID:     oauthClientID,
					ClientSecret: oauthClientSecret,
					BaseURL:      "https://github.example",
					RedirectURI:  oauthRedirectURI,
				},
				OAuthState:    &errOAuthState{saveErr: storeErr},
				OAuthStateTTL: 10 * time.Minute,
			})
			Expect(err).NotTo(HaveOccurred())
			code, body := doReq(svc.Handler(), http.MethodGet, "/oauth/github/start", "", nil)
			Expect(code).To(Equal(http.StatusServiceUnavailable), "body=%s", body)
			env := decodeEnvelope(body)
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInternalError))
		})

		It("fails closed with 503 internal_error on callback Consume", func() {
			storeErr := errors.New("oauth state store unavailable")
			base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
			svc, err := authn.New(authn.Options{
				SigningKey: []byte(testSecret),
				Issuer:     testIssuer,
				TokenTTL:   time.Hour,
				Now:        fixedNow(base),
				Denylist:   authn.NewMemoryDenylist(),
				GitHubOAuth: &authn.GitHubOAuthConfig{
					ClientID:     oauthClientID,
					ClientSecret: oauthClientSecret,
					BaseURL:      "https://github.example",
					RedirectURI:  oauthRedirectURI,
				},
				OAuthState:    &errOAuthState{consumeErr: storeErr},
				OAuthStateTTL: 10 * time.Minute,
			})
			Expect(err).NotTo(HaveOccurred())
			code, body := doReq(svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
				"code":  {"any"},
				"state": {"any-state"},
			}.Encode(), "", nil)
			Expect(code).To(Equal(http.StatusServiceUnavailable), "body=%s", body)
			env := decodeEnvelope(body)
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInternalError))
		})
	})
})

// errOAuthState fails Save and/or Consume so handlers can prove fail-closed 503.
type errOAuthState struct {
	saveErr    error
	consumeErr error
}

func (e *errOAuthState) Save(context.Context, string, time.Time) error {
	if e.saveErr != nil {
		return e.saveErr
	}
	return nil
}

func (e *errOAuthState) Consume(context.Context, string, time.Time) (bool, error) {
	if e.consumeErr != nil {
		return false, e.consumeErr
	}
	return true, nil
}
