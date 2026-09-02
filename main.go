package main

import (
	"log"

	"solid-parakeet-go/servehttp"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	servehttp.ServeHTTP()
}
