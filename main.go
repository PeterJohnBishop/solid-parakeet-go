package main

import (
	"context"
	"log"

	"solid-parakeet-go/pgdb"
	"solid-parakeet-go/servehttp"

	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	db, err := pgdb.ConnectPool(ctx)
	if err != nil {
		log.Fatalf("Error connecting to postgres: %v", err)
	}
	servehttp.ServeHTTP(db)
}
