// package main means this file builds into an executable program.
package main

import (
	// log prints log messages to the console
	"log"
	// net/http provides HTTP client and server implementations
	"net/http"
	
	"github.com/ayMissouri/watchlist-go.git/internal/server"
)

// func main is the entry point of the program.
func main() {
	router := server.NewRouter()

	log.Println("server listening on 8080")

	// errors are handled explicitly in Go instead of relying on exceptions.
	// if ListenAndServe returns an error, it will be logged and the program will exit.
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}