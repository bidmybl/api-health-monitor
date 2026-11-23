package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryCheckLock(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	if databaseURL == os.Getenv("DATABASE_URL") {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repository := NewRepository(pool)
	endpointID := time.Now().UnixNano()

	releaseFirst, acquired, err := repository.TryAcquireCheckLock(
		ctx,
		endpointID,
	)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	if !acquired {
		t.Fatal("first lock was not acquired")
	}

	releaseSecond, acquired, err := repository.TryAcquireCheckLock(
		ctx,
		endpointID,
	)
	if err != nil {
		t.Fatalf("try second lock: %v", err)
	}
	if acquired {
		_ = releaseSecond(ctx)
		t.Fatal("second lock was acquired while first lock was held")
	}

	if err := releaseFirst(ctx); err != nil {
		t.Fatalf("release first lock: %v", err)
	}

	releaseThird, acquired, err := repository.TryAcquireCheckLock(
		ctx,
		endpointID,
	)
	if err != nil {
		t.Fatalf("acquire third lock: %v", err)
	}
	if !acquired {
		t.Fatal("third lock was not acquired after release")
	}

	if err := releaseThird(ctx); err != nil {
		t.Fatalf("release third lock: %v", err)
	}
}
