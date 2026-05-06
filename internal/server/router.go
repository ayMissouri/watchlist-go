package server

import (
	// net/http provides HTTP client and server implementations
	"net/http"
	// time provides functionality for measuring and displaying time.
	"time"

	// chi is a lightweight, idiomatic and composable router for building Go HTTP services.
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/ayMissouri/watchlist-go.git/internal/handlers"
)

// http.Handler is an interface, so this can return anything that represents a HTTP handler.
func NewRouter() http.Handler {
	// chi is a lightweight, idiomatic and composable router for building Go HTTP services.
	// It works with net/http and has a simple API for defining routes and middleware.
	r := chi.NewRouter()

	// r.Use adds middleware to every request.
	// Chi's docs recommend this base stack.
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	r.Get("/health", handlers.Health)

	return r
}