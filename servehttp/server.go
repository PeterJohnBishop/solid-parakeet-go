// Package servehttp
package servehttp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB *pgxpool.Pool
}

// ServeHTTP starts the http server
func ServeHTTP(db *pgxpool.Pool) {
	srv := &Server{DB: db}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", srv.statusHandler)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// run the listener in a background goroutine
	go func() {
		log.Println("Starting server on port :8080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// block main until an interrupt signal (Ctrl+C / SIGTERM) is received
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	// allow active requests up to 5 seconds to finish before forcing close
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// close database pool connections
	db.Close()
	log.Println("Server exiting cleanly")
}

// statusHandler returns the status of the database connection
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	type HealthResponse struct {
		DatabaseConnected bool `json:"database_connected"`
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbConnected := s.DB != nil && s.DB.Ping(ctx) == nil
	w.Header().Set("Content-Type", "application/json")
	if !dbConnected {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(HealthResponse{
		DatabaseConnected: dbConnected,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
