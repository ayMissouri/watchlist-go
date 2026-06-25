package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
)

// OptionalAuth is like RequireAuth but it doesn't return an error if the Authorization header is missing or invalid.
func OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if claims, err := auth.ParseJWT(tokenStr); err == nil {
				ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}
