package middleware

import (
	"net/http"
	"os"
)

func CORS(next http.Handler) http.Handler {
	frontend := os.Getenv("FRONTEND_URL")
	// default to localhost if FRONTEND_URL is not set
	if frontend == "" {
		frontend = "http://localhost:3000"
	}

	allowed := map[string]bool{
		frontend:                 true,
		"https://awatch.fun":     true,
		"https://www.awatch.fun": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !allowed[origin] {
			origin = frontend
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
