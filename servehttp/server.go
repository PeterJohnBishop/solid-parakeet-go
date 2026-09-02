package servehttp

import (
	"log"
	"net/http"
	"os"
)

func ServeHTTP() {
	port := os.Getenv("PORT")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", StatusHandler)
	log.Printf("Server listening on %s", port)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
