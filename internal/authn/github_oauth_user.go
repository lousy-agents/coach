package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func (s *Service) fetchGitHubUser(ctx context.Context, accessToken string) (githubUser, error) {
	base := strings.TrimRight(s.githubOAuth.APIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/user", nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return githubUser{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubUser{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return githubUser{}, fmt.Errorf("user endpoint status %d", resp.StatusCode)
	}
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return githubUser{}, err
	}
	return u, nil
}
