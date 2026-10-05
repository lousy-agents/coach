package authn_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

func body_authnAcceptanceTest_rejectsEachCaseWith401UnauthenticatedWhileAValid_84() {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	now := base
	// trackingDenylist records IsRevoked hits so the jti-denylisted case cannot
	// false-green on expiry alone (Validate checks exp before the denylist).
	mem := authn.NewMemoryDenylist()
	dl := &trackingDenylist{inner: mem}
	svc := newTestService(authn.Options{
		Now:      func() time.Time { return now },
		Denylist: dl,
	})
	h := svc.Handler()

	good, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github",
		Subject:  "12345",
		Login:    "octocat",
	})
	Expect(err).NotTo(HaveOccurred())

	// Denylist a second token after issue (clock stays at base so the token is
	// still unexpired when Validate runs IsRevoked).
	toRevoke, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github",
		Subject:  "99999",
		Login:    "revoked-user",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(svc.Revoke(context.Background(), toRevoke)).To(Succeed())

	other, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     "https://evil.example",
		TokenTTL:   time.Hour,
		Now:        func() time.Time { return base },
		Denylist:   authn.NewMemoryDenylist(),
	})
	Expect(err).NotTo(HaveOccurred())
	wrongIss, err := other.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "1", Login: "x",
	})
	Expect(err).NotTo(HaveOccurred())

	short, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     testIssuer,
		TokenTTL:   time.Minute,
		Now:        func() time.Time { return base },
		Denylist:   authn.NewMemoryDenylist(),
	})
	Expect(err).NotTo(HaveOccurred())
	expiredTok, err := short.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "2", Login: "y",
	})
	Expect(err).NotTo(HaveOccurred())

	type badCase struct {
		name        string
		bearer      string
		advance     time.Duration
		wantRevoked bool
	}
	cases := []badCase{
		{name: "missing Authorization", bearer: ""},
		{name: "invalid signature / garbage", bearer: "not-a-jwt"},
		{name: "wrong issuer", bearer: wrongIss},
		{name: "expired", bearer: expiredTok, advance: 2 * time.Hour},
		{name: "jti denylisted", bearer: toRevoke, wantRevoked: true},
		{name: "github oauth access token stand-in", bearer: "gho_not_a_coach_jwt_at_all"},
	}

	for _, tc := range cases {
		now = base.Add(tc.advance)
		before := dl.isRevokedCalls()
		code, body := doReq(h, http.MethodGet, "/v1/me", tc.bearer, nil)
		expectUnauthenticated(code, body)
		if tc.wantRevoked {
			Expect(dl.isRevokedCalls()).To(BeNumerically(">", before),
				"%s must call IsRevoked (token still unexpired)", tc.name)
			last, ok := dl.lastRevokedResult()
			Expect(ok).To(BeTrue(), "%s: IsRevoked must have recorded a result", tc.name)
			Expect(last).To(BeTrue(), "%s: IsRevoked must report revoked=true", tc.name)
		}
		now = base
	}

	now = base
	code, body := doReq(h, http.MethodGet, "/v1/me", good, nil)
	Expect(code).To(Equal(http.StatusOK), "valid token must authorize /v1/me; body=%s", body)
}
