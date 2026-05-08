package server

import (
	// net/http provides HTTP client and server implementations
	"net/http"
	// time provides functionality for measuring and displaying time.
	"time"

	// chi is a lightweight, idiomatic and composable router for building Go HTTP services.
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/handlers"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
)

// http.Handler is an interface, so this can return anything that represents a HTTP handler.
func NewRouter(database *db.DB) http.Handler {
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

	r.Get("/health", handlers.Health(database))

	authHandler := &handlers.AuthHandler{DB: database}
	wlHandler := &handlers.WatchlistHandler{DB: database}

	// Public auth routes
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/callback", authHandler.Callback)

		// Protected route that requires a valid JWT
		r.With(middleware.RequireAuth).Get("/me", authHandler.Me)
	})

	r.Route("/watchlist", func(r chi.Router) {
		// Applies auth middlware to all watchlist routes.
		r.Use(middleware.RequireAuth)

		r.Get("/", wlHandler.GetAll)
		r.Put("/{id}", wlHandler.Upsert)
		r.Get("/{id}", wlHandler.GetOne)
		r.Patch("/{id}/progress", wlHandler.UpdateProgress)
		r.Delete("/{id}", wlHandler.Delete)
	})

	return r
}
