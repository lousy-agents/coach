package authn_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

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

func newOAuthFake(t *testing.T) (*fakegithub.Fixture, *fakegithub.Server) {
	t.Helper()
	fx := fakegithub.NewFixture("authn-oauth")
	fx.OAuth.ClientID = oauthClientID
	fx.OAuth.ClientSecret = oauthClientSecret
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 42, Login: "octocat"}
	fx.OAuth.Codes[oauthScenarioCode] = fakegithub.OAuthCodeEntry{
		IdentityLogin: "octocat",
		Scenario:      fakegithub.ScenarioOK,
	}
	srv := fakegithub.NewServer(&fx)
	t.Cleanup(srv.Close)
	return &fx, srv
}

func newOAuthService(t *testing.T, githubBase string, now func() time.Time, stateTTL time.Duration) *authn.Service {
	t.Helper()
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
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}
	return svc
}

// Task 2a / Task B: end-to-end fake-GitHub OAuth round-trip mints a Coach JWT
// that authorizes /v1/me; authorize URL requests no scope; GitHub OAuth access
// token is rejected on /v1.
func TestGitHubOAuth_RoundTrip_IssuesCoachJWTThatAuthorizesMe(t *testing.T) {
	_, gh := newOAuthFake(t)
	svc := newOAuthService(t, gh.URL(), nil, 0)
	coach := httptest.NewServer(svc.Handler())
	t.Cleanup(coach.Close)
	client := noRedirectClient()

	// 1) Start OAuth — redirect to GitHub authorize with client_id, redirect_uri, state, no scope.
	startResp, err := client.Get(coach.URL + "/oauth/github/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(startResp.Body)
		t.Fatalf("start status: got %d want 302; body=%s", startResp.StatusCode, body)
	}
	loc := startResp.Header.Get("Location")
	authURL, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse authorize Location: %v (%q)", err, loc)
	}
	if !strings.HasPrefix(loc, gh.URL()+"/login/oauth/authorize") {
		t.Fatalf("authorize Location host/path: got %q want prefix %q", loc, gh.URL()+"/login/oauth/authorize")
	}
	q := authURL.Query()
	if got := q.Get("client_id"); got != oauthClientID {
		t.Errorf("client_id: got %q want %q", got, oauthClientID)
	}
	if got := q.Get("redirect_uri"); got != oauthRedirectURI {
		t.Errorf("redirect_uri: got %q want %q", got, oauthRedirectURI)
	}
	state := q.Get("state")
	if state == "" {
		t.Fatal("state must be non-empty")
	}
	// No scope: empty or absent (GitHub default public identity).
	if _, has := q["scope"]; has {
		if got := q.Get("scope"); strings.TrimSpace(got) != "" {
			t.Errorf("authorize must request no scope; got scope=%q", got)
		}
	}
	if q.Get("scenario_code") != "" {
		t.Error("start must not send scenario_code (fake-only test hook)")
	}

	// 2) Complete authorize at fake GitHub with scenario_code, preserving coach state.
	fakeAuth := gh.URL() + "/login/oauth/authorize?" + url.Values{
		"client_id":     {oauthClientID},
		"redirect_uri":  {oauthRedirectURI},
		"state":         {state},
		"scenario_code": {oauthScenarioCode},
	}.Encode()
	authResp, err := client.Get(fakeAuth)
	if err != nil {
		t.Fatalf("fake authorize: %v", err)
	}
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(authResp.Body)
		t.Fatalf("fake authorize status: got %d want 302; body=%s", authResp.StatusCode, body)
	}
	cbLoc, err := url.Parse(authResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse callback Location: %v", err)
	}
	if cbLoc.Query().Get("code") == "" || cbLoc.Query().Get("state") != state {
		t.Fatalf("callback Location query: code=%q state=%q want state=%q",
			cbLoc.Query().Get("code"), cbLoc.Query().Get("state"), state)
	}

	// 3) Coach callback exchanges code, mints Coach JWT.
	cbPath := "/oauth/github/callback?" + cbLoc.RawQuery
	cbResp, err := client.Get(coach.URL + cbPath)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer cbResp.Body.Close()
	cbBody, _ := io.ReadAll(cbResp.Body)
	if cbResp.StatusCode != http.StatusOK {
		t.Fatalf("callback status: got %d want 200; body=%s", cbResp.StatusCode, cbBody)
	}
	var tokResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(cbBody, &tokResp); err != nil {
		t.Fatalf("callback JSON: %v body=%s", err, cbBody)
	}
	if tokResp.AccessToken == "" {
		t.Fatal("callback access_token must be non-empty")
	}
	if !strings.EqualFold(tokResp.TokenType, "bearer") {
		t.Errorf("token_type: got %q want bearer", tokResp.TokenType)
	}
	// Three JWT segments — not a raw GitHub oauth token.
	if parts := strings.Split(tokResp.AccessToken, "."); len(parts) != 3 {
		t.Fatalf("access_token must be Coach JWT (3 segments); got %d parts", len(parts))
	}

	// 4) Coach JWT authorizes protected /v1/me with github principal.
	meCode, meBody := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tokResp.AccessToken, nil)
	if meCode != http.StatusOK {
		t.Fatalf("/v1/me status: got %d want 200; body=%s", meCode, meBody)
	}
	var p coachapi.Principal
	if err := json.Unmarshal(meBody, &p); err != nil {
		t.Fatalf("/v1/me JSON: %v body=%s", err, meBody)
	}
	want := coachapi.Principal{Provider: "github", Subject: strconv.FormatInt(42, 10), Login: "octocat"}
	if p != want {
		t.Errorf("principal: got %+v want %+v", p, want)
	}

	// 5) GitHub OAuth access token from exchange is NOT accepted on /v1.
	// Capture a real fake OAuth token by exchanging a fresh code after re-seeding.
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
	if err != nil {
		t.Fatalf("direct token exchange: %v", err)
	}
	var ghTok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(exResp.Body).Decode(&ghTok); err != nil {
		exResp.Body.Close()
		t.Fatalf("decode gh token: %v", err)
	}
	exResp.Body.Close()
	if ghTok.AccessToken == "" {
		t.Fatal("expected fake GitHub access_token")
	}
	// Fake /user works with that token without user-scope grant (empty scope on authorize).
	userReq, err := http.NewRequest(http.MethodGet, gh.URL()+"/user", nil)
	if err != nil {
		t.Fatalf("user req: %v", err)
	}
	userReq.Header.Set("Authorization", "Bearer "+ghTok.AccessToken)
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		t.Fatalf("GET /user: %v", err)
	}
	userBody, _ := io.ReadAll(userResp.Body)
	userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /user with oauth token: status=%d body=%s", userResp.StatusCode, userBody)
	}

	rejCode, rejBody := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", ghTok.AccessToken, nil)
	if rejCode != http.StatusUnauthorized {
		t.Fatalf("GitHub oauth token on /v1/me: status=%d want 401; body=%s", rejCode, rejBody)
	}
	env := decodeEnvelope(t, rejBody)
	if env.Error.Code != coachapi.ErrorCodeUnauthenticated {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeUnauthenticated)
	}
}

// Task 2a / Task B: bad/missing/expired OAuth state → 400 invalid_request.
func TestGitHubOAuth_Callback_RejectsBadState(t *testing.T) {
	_, gh := newOAuthFake(t)
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	now := base
	svc := newOAuthService(t, gh.URL(), func() time.Time { return now }, time.Minute)
	h := svc.Handler()

	// Seed a valid state via start, then expire the clock before callback.
	client := noRedirectClient()
	coach := httptest.NewServer(h)
	t.Cleanup(coach.Close)

	startResp, err := client.Get(coach.URL + "/oauth/github/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("start status: %d", startResp.StatusCode)
	}
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	goodState := authURL.Query().Get("state")
	if goodState == "" {
		t.Fatal("empty state from start")
	}

	// Obtain a real code from fake authorize for the expired-state case.
	authResp, err := client.Get(gh.URL() + "/login/oauth/authorize?" + url.Values{
		"client_id":     {oauthClientID},
		"redirect_uri":  {oauthRedirectURI},
		"state":         {goodState},
		"scenario_code": {oauthScenarioCode},
	}.Encode())
	if err != nil {
		t.Fatalf("fake authorize: %v", err)
	}
	authResp.Body.Close()
	code := ""
	if authResp.StatusCode == http.StatusFound {
		u, _ := url.Parse(authResp.Header.Get("Location"))
		code = u.Query().Get("code")
	}
	if code == "" {
		t.Fatal("expected code from fake authorize")
	}

	cases := []struct {
		name  string
		query url.Values
		setup func()
	}{
		{
			name:  "missing state",
			query: url.Values{"code": {code}},
		},
		{
			name:  "unknown state",
			query: url.Values{"code": {code}, "state": {"never-issued-state"}},
		},
		{
			name:  "expired state",
			query: url.Values{"code": {code}, "state": {goodState}},
			setup: func() { now = base.Add(2 * time.Minute) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now = base
			if tc.setup != nil {
				tc.setup()
			}
			t.Cleanup(func() { now = base })

			codeStatus, body := doReq(t, h, http.MethodGet, "/oauth/github/callback?"+tc.query.Encode(), "", nil)
			if codeStatus != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400; body=%s", codeStatus, body)
			}
			env := decodeEnvelope(t, body)
			if env.Error.Code != coachapi.ErrorCodeInvalidRequest {
				t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeInvalidRequest)
			}
		})
	}
}

// Task B reviewer finding: real GitHub OAuth uses github.com for authorize/token
// but api.github.com for GET /user. When APIBaseURL differs from BaseURL, user
// fetch must hit the API host (not BaseURL+/user).
func TestGitHubOAuth_UserFetch_UsesAPIBaseURL(t *testing.T) {
	const (
		ghAccessToken = "gho_split_host_token"
		ghUserID      = int64(99)
		ghLogin       = "api-host-user"
	)

	var tokenHits, userHits int

	// Server A: OAuth authorize + token exchange only (github.com role).
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/login/oauth/authorize":
			// Not used by callback path; present so BaseURL looks like a real OAuth origin.
			http.Error(w, "authorize not used in this test", http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
			tokenHits++
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
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
			// Must not be called when APIBaseURL is a distinct host.
			t.Errorf("GET /user hit OAuth BaseURL host; want APIBaseURL")
			http.Error(w, "wrong host for /user", http.StatusNotFound)
		default:
			http.Error(w, "not found on oauth host: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(oauthSrv.Close)

	// Server B: REST API GET /user only (api.github.com role).
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/user" {
			http.Error(w, "not found on api host: "+r.URL.Path, http.StatusNotFound)
			return
		}
		userHits++
		authz := r.Header.Get("Authorization")
		if authz != "Bearer "+ghAccessToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    ghUserID,
			"login": ghLogin,
		})
	}))
	t.Cleanup(apiSrv.Close)

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
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}

	// Seed CSRF state via start, then invoke callback with a synthetic code.
	client := noRedirectClient()
	coach := httptest.NewServer(svc.Handler())
	t.Cleanup(coach.Close)

	startResp, err := client.Get(coach.URL + "/oauth/github/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("start status: got %d want 302", startResp.StatusCode)
	}
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatal("empty state from start")
	}
	// Authorize URL must target OAuth BaseURL, not API base.
	if !strings.HasPrefix(startResp.Header.Get("Location"), oauthSrv.URL+"/login/oauth/authorize") {
		t.Fatalf("authorize Location: got %q want prefix %q", startResp.Header.Get("Location"), oauthSrv.URL+"/login/oauth/authorize")
	}

	cbResp, err := client.Get(coach.URL + "/oauth/github/callback?" + url.Values{
		"code":  {"split-host-code"},
		"state": {state},
	}.Encode())
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer cbResp.Body.Close()
	cbBody, _ := io.ReadAll(cbResp.Body)
	if cbResp.StatusCode != http.StatusOK {
		t.Fatalf("callback status: got %d want 200; body=%s", cbResp.StatusCode, cbBody)
	}
	var tokResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(cbBody, &tokResp); err != nil {
		t.Fatalf("callback JSON: %v body=%s", err, cbBody)
	}
	if tokResp.AccessToken == "" {
		t.Fatal("callback access_token must be non-empty")
	}
	if tokenHits != 1 {
		t.Errorf("token endpoint hits on BaseURL: got %d want 1", tokenHits)
	}
	if userHits != 1 {
		t.Errorf("GET /user hits on APIBaseURL: got %d want 1", userHits)
	}

	meCode, meBody := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tokResp.AccessToken, nil)
	if meCode != http.StatusOK {
		t.Fatalf("/v1/me status: got %d want 200; body=%s", meCode, meBody)
	}
	var p coachapi.Principal
	if err := json.Unmarshal(meBody, &p); err != nil {
		t.Fatalf("/v1/me JSON: %v body=%s", err, meBody)
	}
	want := coachapi.Principal{Provider: "github", Subject: strconv.FormatInt(ghUserID, 10), Login: ghLogin}
	if p != want {
		t.Errorf("principal: got %+v want %+v", p, want)
	}
}

// Incomplete GitHub /user payloads (zero id or empty login alone) must 400 and
// not mint a Coach token — both sides of the incomplete check are required.
func TestGitHubOAuth_Callback_RejectsIncompleteUser(t *testing.T) {
	cases := []struct {
		name string
		user map[string]any
	}{
		{name: "zero_id_with_login", user: map[string]any{"id": 0, "login": "ghost"}},
		{name: "empty_login_with_id", user: map[string]any{"id": int64(7), "login": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
					_ = json.NewEncoder(w).Encode(tc.user)
				default:
					http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
				}
			}))
			t.Cleanup(gh.Close)

			svc := newOAuthService(t, gh.URL, nil, 0)
			client := noRedirectClient()
			coach := httptest.NewServer(svc.Handler())
			t.Cleanup(coach.Close)

			startResp, err := client.Get(coach.URL + "/oauth/github/start")
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			startResp.Body.Close()
			if startResp.StatusCode != http.StatusFound {
				t.Fatalf("start status: got %d want 302", startResp.StatusCode)
			}
			authURL, err := url.Parse(startResp.Header.Get("Location"))
			if err != nil {
				t.Fatalf("parse Location: %v", err)
			}
			state := authURL.Query().Get("state")
			if state == "" {
				t.Fatal("empty state")
			}

			status, body := doReq(t, svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
				"code":  {"incomplete-user-code"},
				"state": {state},
			}.Encode(), "", nil)
			if status != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400; body=%s", status, body)
			}
			env := decodeEnvelope(t, body)
			if env.Error.Code != coachapi.ErrorCodeInvalidRequest {
				t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeInvalidRequest)
			}
			if strings.Contains(string(body), "access_token") {
				t.Errorf("incomplete user must not issue a token; body=%s", body)
			}
		})
	}
}

// OAuth CSRF state is single-use: a second callback with the same state must fail
// even when a fresh authorization code is presented.
func TestGitHubOAuth_Callback_RejectsReusedState(t *testing.T) {
	fx, gh := newOAuthFake(t)
	fx.OAuth.Codes["code-second"] = fakegithub.OAuthCodeEntry{
		IdentityLogin: "octocat",
		Scenario:      fakegithub.ScenarioOK,
	}
	svc := newOAuthService(t, gh.URL(), nil, 0)
	client := noRedirectClient()
	coach := httptest.NewServer(svc.Handler())
	t.Cleanup(coach.Close)

	startResp, err := client.Get(coach.URL + "/oauth/github/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("start status: got %d want 302", startResp.StatusCode)
	}
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatal("empty state")
	}

	// First callback succeeds and consumes state.
	status, body := doReq(t, svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
		"code":  {oauthScenarioCode},
		"state": {state},
	}.Encode(), "", nil)
	if status != http.StatusOK {
		t.Fatalf("first callback: got %d want 200; body=%s", status, body)
	}
	var first struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &first); err != nil || first.AccessToken == "" {
		t.Fatalf("first callback token: err=%v body=%s", err, body)
	}

	// Same state + unused code must not mint another token.
	status, body = doReq(t, svc.Handler(), http.MethodGet, "/oauth/github/callback?"+url.Values{
		"code":  {"code-second"},
		"state": {state},
	}.Encode(), "", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("reused state: got %d want 400; body=%s", status, body)
	}
	env := decodeEnvelope(t, body)
	if env.Error.Code != coachapi.ErrorCodeInvalidRequest {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeInvalidRequest)
	}
	if strings.Contains(string(body), "access_token") {
		t.Errorf("reused state must not issue a token; body=%s", body)
	}
}

// Task 2a / Task B: GitHub error=access_denied on callback → 400, no Coach token.
func TestGitHubOAuth_Callback_AccessDenied_NoToken(t *testing.T) {
	_, gh := newOAuthFake(t)
	svc := newOAuthService(t, gh.URL(), nil, 0)
	h := svc.Handler()

	// Still need a valid state so we prove we reject on error param before/with state checks.
	client := noRedirectClient()
	coach := httptest.NewServer(h)
	t.Cleanup(coach.Close)
	startResp, err := client.Get(coach.URL + "/oauth/github/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startResp.Body.Close()
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := authURL.Query().Get("state")

	q := url.Values{
		"error":             {"access_denied"},
		"error_description": {"The user has denied your application access."},
		"state":             {state},
	}
	status, body := doReq(t, h, http.MethodGet, "/oauth/github/callback?"+q.Encode(), "", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400; body=%s", status, body)
	}
	env := decodeEnvelope(t, body)
	if env.Error.Code != coachapi.ErrorCodeInvalidRequest {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeInvalidRequest)
	}
	// Must not look like a token response.
	if strings.Contains(string(body), "access_token") {
		t.Errorf("access_denied callback must not issue a token; body=%s", body)
	}
}
