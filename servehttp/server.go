// Package servehttp
package servehttp

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

type Server struct {
	DB *sql.DB
}

// ServeHTTP starts the http server
func ServeHTTP(db *sql.DB) {
	srv := &Server{DB: db}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", srv.statusHandler)

	log.Println("Starting server on port :8080")
	http.ListenAndServe(":8080", mux)
}

// statusHandler returns the status of the database connection
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	type HealthResponse struct {
		DatabaseConnected bool `json:"database_connected"`
	}

	dbStatus := s.DB != nil

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(HealthResponse{
		DatabaseConnected: dbStatus,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
