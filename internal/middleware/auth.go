package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
)

// contextKey is our own type so nothing else stuffing values into the ctx can collide with ours.
type contextKey string

const UserClaimsKey contextKey = "userClaims"

// RequireAuth checks the Bearer JWT and puts the claims on the request context.
// No valid token, no handler: the request stops here with a 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")

		claims, err := auth.ParseJWT(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// Handlers pull these back out with ClaimsFromCtx.
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromCtx returns the claims RequireAuth/OptionalAuth stored, or nil if nobody's logged in.
func ClaimsFromCtx(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(UserClaimsKey).(*auth.Claims)
	return claims
}