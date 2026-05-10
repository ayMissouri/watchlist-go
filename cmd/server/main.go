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
	
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/server"
	"github.com/ayMissouri/watchlist-go.git/internal/auth"
)

// func main is the entry point of the program.
func main() {
	// Load .env (ignored if not present)
	_ = godotenv.Load()
	auth.InitDiscord()

	// context.Background() is the root context — used at startup
	// where there's no incoming request to derive a context from
	ctx := context.Background()

	database, err := db.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("could not connect to db: %v", err)
	}

	router := server.NewRouter(database)

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