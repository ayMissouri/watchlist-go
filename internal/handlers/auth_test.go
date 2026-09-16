package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestUpdateUserRequestPartialFields(t *testing.T) {
	var settingsOnly models.UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"settings":{"subtitle":{"size":18}}}`), &settingsOnly); err != nil {
		t.Fatal(err)
	}
	if settingsOnly.DisplayName != nil {
		t.Errorf("display_name should be untouched, got %q", *settingsOnly.DisplayName)
	}
	if string(settingsOnly.Settings) == "" {
		t.Error("settings not decoded")
	}

	var clearName models.UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"display_name":""}`), &clearName); err != nil {
		t.Fatal(err)
	}
	if clearName.DisplayName == nil || *clearName.DisplayName != "" {
		t.Error("empty display_name should be present and empty")
	}
	if clearName.Settings != nil {
		t.Error("settings should be untouched")
	}
}

func TestLoginNativeRedirectCookie(t *testing.T) {
	auth.InitDiscord()
	h := &AuthHandler{}

	cookie := func(target string) *http.Cookie {
		rec := httptest.NewRecorder()
		h.Login(rec, httptest.NewRequest(http.MethodGet, target, nil))
		for _, c := range rec.Result().Cookies() {
			if c.Name == "oauth_redirect" {
				return c
			}
		}
		t.Fatalf("no oauth_redirect cookie for %s", target)
		return nil
	}

	allowed := cookie("/auth/login?redirect_uri=" + url.QueryEscape("awatch://auth/callback"))
	if allowed.Value != "awatch://auth/callback" || allowed.MaxAge <= 0 {
		t.Errorf("allow-listed redirect not stored: %+v", allowed)
	}

	for _, target := range []string{
		"/auth/login?redirect_uri=" + url.QueryEscape("evil://auth/callback"),
		"/auth/login?redirect_uri=" + url.QueryEscape("awatch://auth/callback/../elsewhere"),
		"/auth/login",
	} {
		if c := cookie(target); c.Value != "" || c.MaxAge != -1 {
			t.Errorf("%s should clear the cookie, got %+v", target, c)
		}
	}
}

func TestReviewLoginRejects(t *testing.T) {
	h := &AuthHandler{}
	post := func(body string) int {
		rec := httptest.NewRecorder()
		h.ReviewLogin(rec, httptest.NewRequest(http.MethodPost, "/auth/review-login", strings.NewReader(body)))
		return rec.Code
	}

	t.Setenv("REVIEW_EMAIL", "")
	t.Setenv("REVIEW_PASSWORD", "")
	if c := post(`{"email":"","password":""}`); c != http.StatusNotFound {
		t.Errorf("unconfigured: want 404, got %d", c)
	}

	t.Setenv("REVIEW_EMAIL", "review@example.com")
	t.Setenv("REVIEW_PASSWORD", "secret")
	for body, want := range map[string]int{
		`not json`: http.StatusBadRequest,
		`{"email":"review@example.com","password":"wrong"}`: http.StatusUnauthorized,
		`{"email":"other@example.com","password":"secret"}`: http.StatusUnauthorized,
		`{"email":"review@example.com","password":""}`:      http.StatusUnauthorized,
	} {
		if c := post(body); c != want {
			t.Errorf("%s: want %d, got %d", body, want, c)
		}
	}
}
