// @title           Watchlist API
// @version         1.0
// @description     Backend API for tracking movies and TV shows.

// @contact.name    ayMissouri
// @contact.url     https://github.com/ayMissouri/watchlist-go.git

// @license.name    MIT

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token as: Bearer {token}
package main
// package main means this file builds into an executable program.

import (
	"context"
	// log prints log messages to the console
	"log"
	// net/http provides HTTP client and server implementations
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
	"github.com/ayMissouri/watchlist-go.git/internal/calendar"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/server"
	"github.com/ayMissouri/watchlist-go.git/migrations"
)

// func main is the entry point of the program.
func main() {
	// Load .env (ignored if not present)
	_ = godotenv.Load()
	auth.InitDiscord()

	// context.Background() is the root context thats used at startup
	// where there's no incoming request to derive a context from
	ctx := context.Background()

	database, err := db.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("could not connect to db: %v", err)
	}

	if err := database.Migrate(ctx, migrations.FS); err != nil {
		log.Fatalf("could not run migrations: %v", err)
	}

	metaClient := meta.NewClient()

	// Start the daily calendar job in the background unless DISABLE_CALENDAR_JOB is true.
	if os.Getenv("DISABLE_CALENDAR_JOB") != "true" {
		go calendar.NewService(database, metaClient).RunDaily(ctx)
	}

	router := server.NewRouter(database, metaClient)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server listening on :%s", port)

	// errors are handled explicitly in Go instead of relying on exceptions.
	// if ListenAndServe returns an error, it will be logged and the program will exit.
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}