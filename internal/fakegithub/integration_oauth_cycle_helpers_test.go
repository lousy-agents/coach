package fakegithub_test

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

type concurrentOAuthOutcome struct {
	err       error
	status    int
	login     string
	id        int64
	authorize int
	exchange  int
}

func runConcurrentOAuthCycle(server *fakegithub.Server, i int) concurrentOAuthOutcome {
	code := fmt.Sprintf("concurrent-code-%d", i)
	out := concurrentOAuthOutcome{}

	authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
		"client_id":     {"integration-client-id"},
		"redirect_uri":  {"https://coach.example.com/callback"},
		"state":         {"xyz-state"},
		"scenario_code": {code},
	}.Encode()
	authorizeResp, err := noRedirectClient().Get(authorizeURL)
	if err != nil {
		out.err = fmt.Errorf("authorize: %w", err)
		return out
	}
	authorizeResp.Body.Close()
	out.authorize = authorizeResp.StatusCode

	exchangeResp, err := http.PostForm(server.URL()+"/login/oauth/access_token", url.Values{
		"client_id":     {"integration-client-id"},
		"client_secret": {"integration-client-secret"},
		"code":          {code},
	})
	if err != nil {
		out.err = fmt.Errorf("exchange: %w", err)
		return out
	}
	out.exchange = exchangeResp.StatusCode
	var exchangeBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := decodeResponse(exchangeResp, &exchangeBody); err != nil {
		out.err = fmt.Errorf("decode exchange body: %w", err)
		return out
	}

	userReq, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
	if err != nil {
		out.err = fmt.Errorf("new /user request: %w", err)
		return out
	}
	userReq.Header.Set("Authorization", "token "+exchangeBody.AccessToken)
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		out.err = fmt.Errorf("/user: %w", err)
		return out
	}
	out.status = userResp.StatusCode
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := decodeResponse(userResp, &user); err != nil {
		out.err = fmt.Errorf("decode /user body: %w", err)
		return out
	}
	out.login = user.Login
	out.id = user.ID
	return out
}

func decodeResponse(resp *http.Response, dest any) error {
	defer resp.Body.Close()
	return jsonDecode(resp, dest)
}
