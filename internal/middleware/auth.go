package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
)

// contextKey is an unexported type for context keys in this package.
// Using a custom type prevents key collisions with other packages that also store values in context.
type contextKey string

const UserClaimsKey contextKey = "userClaims"

// 1. RequireAuth is a middleware that validates the Bearer JWT on every request.
// 2. Middleware in Chi is just a function that wraps a http.Handler,
// it recieves the next handler, wraps it with logic, and calls next.ServeHTTP to continue the chain.
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

		// context.WithValue attaches a value to the request context, so handlers further down can access it.
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromCtx is a helper that makes it so helpers dont need to know about the context key type directly.
func ClaimsFromCtx(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(UserClaimsKey).(*auth.Claims)
	return claims
}