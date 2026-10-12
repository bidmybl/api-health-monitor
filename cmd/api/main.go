package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer conn.Close(context.Background())

	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	fmt.Println("Successfully connected to db!")
}
