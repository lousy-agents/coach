package authn_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

const (
	testIssuer = "https://coach.test"
	testSecret = "test-signing-secret-at-least-32-bytes!!"
)

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newTestService(opts authn.Options) *authn.Service {
	if opts.SigningKey == nil {
		opts.SigningKey = []byte(testSecret)
	}
	if opts.Issuer == "" {
		opts.Issuer = testIssuer
	}
	if opts.TokenTTL == 0 {
		opts.TokenTTL = time.Hour
	}
	if opts.Now == nil {
		opts.Now = fixedNow(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	}
	if opts.Denylist == nil {
		opts.Denylist = authn.NewMemoryDenylist()
	}
	svc, err := authn.New(opts)
	Expect(err).NotTo(HaveOccurred())
	return svc
}

func decodeEnvelope(body []byte) coachapi.ErrorEnvelope {
	var env coachapi.ErrorEnvelope
	Expect(json.Unmarshal(body, &env)).To(Succeed(), "body=%s", body)
	return env
}

func doReq(h http.Handler, method, path, bearer string, body []byte) (int, []byte) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func expectUnauthenticated(code int, body []byte) {
	Expect(code).To(Equal(http.StatusUnauthorized), "body=%s", body)
	env := decodeEnvelope(body)
	Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeUnauthenticated))
	Expect(strings.TrimSpace(env.Error.Message)).NotTo(BeEmpty())
}
