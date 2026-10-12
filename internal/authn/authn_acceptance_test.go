package authn_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

var _ = Describe("Coach JWT auth on protected /v1 routes", func() {
	When("the bearer is missing, invalid, wrong-issuer, expired, denylisted, or a GitHub OAuth stand-in", func() {
		It("rejects each case with 401 unauthenticated, while a valid Coach JWT authorizes /v1/me", func() {
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
		})
	})

	When("the denylist store errors during Validate", func() {
		It("fails closed with 503 internal_error, distinct from denylisted-jti → 401", func() {
			dl := &errDenylist{err: errors.New("denylist unavailable")}
			svc := newTestService(authn.Options{Denylist: dl})
			tok, err := svc.Issue(context.Background(), coachapi.Principal{
				Provider: "github", Subject: "1", Login: "octocat",
			})
			Expect(err).NotTo(HaveOccurred())

			code, body := doReq(svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
			Expect(code).To(Equal(http.StatusServiceUnavailable), "body=%s", body)
			env := decodeEnvelope(body)
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInternalError))
		})
	})

	When("test-mint is disabled (the default)", func() {
		It("leaves the path unregistered and returns 404 not_found", func() {
			svc := newTestService(authn.Options{TestMintEnabled: false})
			body := []byte(`{"subject":"12345","login":"octocat"}`)
			code, resp := doReq(svc.Handler(), http.MethodPost, "/v1/auth/test-mint", "", body)
			Expect(code).To(Equal(http.StatusNotFound), "body=%s", resp)
			env := decodeEnvelope(resp)
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeNotFound))
		})
	})

	When("test-mint is enabled", func() {
		It("issues a JWT that authorizes /v1/me with Principal matching the mint request", func() {
			svc := newTestService(authn.Options{TestMintEnabled: true})
			h := svc.Handler()

			mintBody := []byte(`{"subject":"424242","login":"hubot"}`)
			code, resp := doReq(h, http.MethodPost, "/v1/auth/test-mint", "", mintBody)
			Expect(code).To(Equal(http.StatusOK), "body=%s", resp)
			var mintResp struct {
				Token string `json:"token"`
			}
			Expect(json.Unmarshal(resp, &mintResp)).To(Succeed(), "body=%s", resp)
			Expect(mintResp.Token).NotTo(BeEmpty())

			code, meBody := doReq(h, http.MethodGet, "/v1/me", mintResp.Token, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", meBody)
			var p coachapi.Principal
			Expect(json.Unmarshal(meBody, &p)).To(Succeed(), "body=%s", meBody)
			Expect(p).To(Equal(coachapi.Principal{Provider: "github", Subject: "424242", Login: "hubot"}))
		})
	})

	When("a Coach JWT is issued", func() {
		It("carries provider, sub, login, iss, exp, and jti, and validates to the same Principal", func() {
			svc := newTestService(authn.Options{})
			in := coachapi.Principal{Provider: "github", Subject: "12345", Login: "octocat"}
			tok, err := svc.Issue(context.Background(), in)
			Expect(err).NotTo(HaveOccurred())
			Expect(tok).NotTo(BeEmpty())
			Expect(strings.Split(tok, ".")).To(HaveLen(3), "token must be compact JWT with 3 segments")

			got, err := svc.Validate(context.Background(), tok)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(in))

			claims, err := svc.InspectClaims(tok)
			Expect(err).NotTo(HaveOccurred())
			Expect(claims.Provider).To(Equal("github"))
			Expect(claims.Subject).To(Equal("12345"))
			Expect(claims.Login).To(Equal("octocat"))
			Expect(claims.Issuer).To(Equal(testIssuer))
			Expect(claims.ExpiresAt.IsZero()).To(BeFalse(), "exp must be set")
			Expect(claims.ID).NotTo(BeEmpty(), "jti must be set")
		})
	})

	When("Now equals the token's exp instant", func() {
		It("rejects the token (expiry is exclusive at exp), while exp-1ns still authorizes", func() {
			base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
			now := base
			const ttl = time.Hour
			svc := newTestService(authn.Options{
				TokenTTL: ttl,
				Now:      func() time.Time { return now },
			})
			tok, err := svc.Issue(context.Background(), coachapi.Principal{
				Provider: "github", Subject: "9", Login: "boundary",
			})
			Expect(err).NotTo(HaveOccurred())

			now = base.Add(ttl - time.Nanosecond)
			_, err = svc.Validate(context.Background(), tok)
			Expect(err).NotTo(HaveOccurred())
			code, body := doReq(svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)

			now = base.Add(ttl)
			_, err = svc.Validate(context.Background(), tok)
			Expect(err).To(HaveOccurred(), "Validate at exp==now must fail")
			code, body = doReq(svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
			expectUnauthenticated(code, body)
		})
	})

	When("a token is revoked", func() {
		It("denylists jti so subsequent Validate and /v1/me fail with 401 unauthenticated", func() {
			svc := newTestService(authn.Options{})
			tok, err := svc.Issue(context.Background(), coachapi.Principal{
				Provider: "github", Subject: "7", Login: "revokee",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Revoke(context.Background(), tok)).To(Succeed())
			_, err = svc.Validate(context.Background(), tok)
			Expect(err).To(HaveOccurred(), "Validate after Revoke must fail")
			code, body := doReq(svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
			expectUnauthenticated(code, body)
		})
	})

	When("the request method does not match a registered /v1 route", func() {
		It("returns an enveloped 404 not_found with application/json", func() {
			svc := newTestService(authn.Options{})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/me", nil)
			svc.Handler().ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusNotFound), "body=%s", rec.Body.String())
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
			env := decodeEnvelope(rec.Body.Bytes())
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeNotFound))
		})
	})
})
