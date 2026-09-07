package server

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/ayMissouri/watchlist-go.git/docs"
	"github.com/ayMissouri/watchlist-go.git/internal/calendar"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/handlers"
	"github.com/ayMissouri/watchlist-go.git/internal/lobby"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

func NewRouter(database *db.DB, metaClient *meta.Client) http.Handler {
	r := chi.NewRouter()

	// The usual chi stack, applied to everything.
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS)

	tracker := tracking.NewService(database, metaClient)
	calendarSvc := calendar.NewService(database, metaClient)

	authHandler := &handlers.AuthHandler{DB: database, Tracker: tracker}
	wlHandler := &handlers.WatchlistHandler{DB: database, Tracker: tracker, Calendar: calendarSvc}
	notifHandler := &handlers.NotificationsHandler{DB: database, Calendar: calendarSvc}
	discoverHandler := &handlers.DiscoverHandler{
		Meta:    metaClient,
		Tracker: tracker,
	}
	eventsHandler := &handlers.EventsHandler{DB: database, Tracker: tracker}
	statsHandler := &handlers.StatsHandler{Tracker: tracker}
	calendarHandler := &handlers.CalendarHandler{
		DB:       database,
		Calendar: calendarSvc,
	}
	lobbyHandler := &handlers.LobbyHandler{DB: database, Lobbies: lobby.NewManager()}

	r.With(middleware.RequireAuth).Get("/lobbies/{code}/stream", lobbyHandler.Stream)

	r.Group(func(r chi.Router) {
		r.Use(chimiddleware.Timeout(30 * time.Second))

		if os.Getenv("ENV") != "production" {
			r.Get("/swagger/*", httpSwagger.Handler(
				httpSwagger.URL("/swagger/doc.json"),
			))
		}

		r.Get("/health", handlers.Health(database))

		// Public auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", authHandler.Login)
			r.Get("/callback", authHandler.Callback)

			r.With(middleware.RequireAuth).Get("/me", authHandler.Me)
			r.With(middleware.RequireAuth).Patch("/me", authHandler.UpdateMe)
		})

		r.Route("/watchlist", func(r chi.Router) {
			// Everything in here needs a valid token.
			r.Use(middleware.RequireAuth)

			r.Get("/", wlHandler.GetAll)
			r.Put("/{id}", wlHandler.Upsert)
			r.Get("/{id}", wlHandler.GetOne)
			r.Patch("/{id}/progress", wlHandler.UpdateProgress)
			r.Patch("/{id}/status", wlHandler.UpdateStatus)
			r.Post("/{id}/plays", wlHandler.AddPlay)
			r.Delete("/{id}/plays", wlHandler.RemovePlay)
			r.Delete("/{id}", wlHandler.Delete)
			r.Delete("/", wlHandler.BulkDelete)
		})

		r.Route("/notifications", func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/", notifHandler.GetAll)
			r.Patch("/read-all", notifHandler.MarkAllRead)
			r.Patch("/{id}/read", notifHandler.MarkRead)
		})

		r.Route("/calendar", func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/", calendarHandler.GetAll)
			r.Post("/refresh", calendarHandler.Refresh)
		})

		r.Route("/stats", func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/", statsHandler.Profile)
			r.Get("/wrapped", statsHandler.Wrapped)
		})

		r.Route("/events", func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/", eventsHandler.List)
			r.Post("/", eventsHandler.Record)
		})

		r.Route("/lobbies", func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Post("/", lobbyHandler.Create)
			r.Post("/{code}/state", lobbyHandler.Update)
		})

		// Public browse routes
		r.Route("/discover", func(r chi.Router) {
			r.Use(middleware.OptionalAuth)

			r.Get("/", discoverHandler.Discover)
			r.Get("/all", discoverHandler.DiscoverAll)
		})

		r.Route("/meta", func(r chi.Router) {
			r.Use(middleware.OptionalAuth)

			r.Get("/movie/{id}", discoverHandler.MovieDetail)
			r.Get("/series/{id}", discoverHandler.SeriesDetail)
			r.Get("/person/{id}", discoverHandler.PersonDetail)
			r.Get("/{type}/{id}/recommendations", discoverHandler.Recommendations)
		})

		r.With(middleware.OptionalAuth).Get("/search", discoverHandler.Search)
	})

	return r
}
