package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowedOrigins(t *testing.T) {
	t.Setenv("FRONTEND_URL", "http://localhost:3000")
	h := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	cases := map[string]string{
		"https://awatch.fun":     "https://awatch.fun",
		"https://www.awatch.fun": "https://www.awatch.fun",
		"http://localhost:3000":  "http://localhost:3000",
		"https://evil.example":   "http://localhost:3000",
	}
	for origin, want := range cases {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("origin %s: got %q, want %q", origin, got, want)
		}
	}
}
