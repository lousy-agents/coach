package authn_test

import (
	"context"

	"net/http"

	"time"
)

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

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
