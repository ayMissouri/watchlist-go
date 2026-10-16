package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
	"github.com/ayMissouri/watchlist-go.git/internal/gemini"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
)

func TestWindowLimiter(t *testing.T) {
	l := newWindowLimiter(2, time.Minute)
	now := time.Now()
	for i, want := range []bool{true, true, false} {
		if got := l.allow("ip:1.2.3.4", now); got != want {
			t.Errorf("call %d: got %v", i+1, got)
		}
	}
	if !l.allow("ip:5.6.7.8", now) {
		t.Error("another caller has its own window")
	}
	if !l.allow("ip:1.2.3.4", now.Add(time.Minute)) {
		t.Error("a new window starts after a minute")
	}
}

func TestAMIErrorCodes(t *testing.T) {
	cases := []struct {
		err    error
		code   string
		status int
	}{
		{&gemini.Error{Kind: gemini.Quota, RetryAfter: 36 * time.Second}, "crowded", 429},
		{&gemini.Error{Kind: gemini.Quota}, "exhausted", 429},
		{&gemini.Error{Kind: gemini.Blocked}, "blocked", 422},
		{&gemini.Error{Kind: gemini.Unauthorized}, "unavailable", 503},
		{&gemini.Error{Kind: gemini.Unavailable}, "busy", 503},
		{errors.New("context deadline exceeded"), "busy", 503},
	}
	for _, c := range cases {
		if code, status := amiError(c.err); code != c.code || status != c.status {
			t.Errorf("%v: got %s %d", c.err, code, status)
		}
	}
}

func TestAMIGenerateChecksSetupAndBody(t *testing.T) {
	post := func(h *AMIHandler, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.Generate(rec, httptest.NewRequest(http.MethodPost, "/ami", strings.NewReader(body)))
		return rec
	}

	if rec := post(NewAMIHandler(nil, gemini.NewClient("")), `{}`); rec.Code != 503 || !strings.Contains(rec.Body.String(), "unavailable") {
		t.Errorf("no key: got %d %s", rec.Code, rec.Body)
	}
	h := NewAMIHandler(nil, gemini.NewClient("test-key"))
	for _, body := range []string{`nope`, `{"prompt":"hi","schema":{}}`, `{"instructions":"i","prompt":"hi","schema":"x"}`} {
		if rec := post(h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d", body, rec.Code)
		}
	}
}

func TestAMIGenerateNeedsWatchAccess(t *testing.T) {
	h := NewAMIHandler(nil, gemini.NewClient("test-key"))
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/ami", strings.NewReader(`{"instructions":"i","prompt":"hi","schema":{"type":"object"}}`))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserClaimsKey, &auth.Claims{UserID: "u1"}))
		rec := httptest.NewRecorder()
		h.Generate(rec, req)
		return rec
	}

	h.hasAccess = func(context.Context, string) (bool, error) { return false, nil }
	if rec := send(); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "no_access") {
		t.Errorf("no access: got %d %s", rec.Code, rec.Body)
	}
	h.hasAccess = func(context.Context, string) (bool, error) { return false, errors.New("db down") }
	if rec := send(); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("lookup failed: got %d %s", rec.Code, rec.Body)
	}
}
