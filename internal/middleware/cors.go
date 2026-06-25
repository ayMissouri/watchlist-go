package middleware

import (
	"net/http"
	"os"
)

func CORS(next http.Handler) http.Handler {
	origin := os.Getenv("FRONTEND_URL")
	// default to localhost if FRONTEND_URL is not set
	if origin == "" {
		origin = "http://localhost:3000"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
