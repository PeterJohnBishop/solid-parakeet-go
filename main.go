package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"solid-parakeet-go/pgdb"
	"solid-parakeet-go/servehttp"

	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	db, err := pgdb.ConnectPool(ctx)
	if err != nil {
		log.Fatalf("Error connecting to postgres: %v", err)
	}

	pgdb.ConsumeGTFS(ctx, db)

	servehttp.ServeHTTP(db)
}
