// package main means this file builds into an executable program.
package main

import (
	// fmt writes formatted output to the console
	"fmt"
	// log prints log messages to the console
	"log"
	// net/http provides HTTP client and server implementations
	"net/http"
)

// this is a named function that handles HTTP requests to the /health endpoint.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}

// func main is the entry point of the program.
func main() {
	http.HandleFunc("/health", healthHandler)

	log.Println("server listening on :8080")

	// errors are handled explicitly in Go instead of relying on exceptions.
	// if ListenAndServe returns an error, it will be logged and the program will exit.
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}