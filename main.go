package main

import (
	"log"

	"solid-parakeet-go/pgdb"
	"solid-parakeet-go/servehttp"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	db, err := pgdb.ConnectDB()
	if err != nil {
		log.Fatalf("Error connecting to postgres: %v", err)
	}
	servehttp.ServeHTTP(db)
}
