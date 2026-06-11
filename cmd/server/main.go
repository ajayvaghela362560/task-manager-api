package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskmanager/internal/api"
	"taskmanager/internal/realtime"
	"taskmanager/internal/store"
)

func main() {
	cfg := api.LoadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) == 0 {
		log.Fatal("JWT_SECRET is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool := connectWithRetry(ctx, cfg.DatabaseURL)
	defer pool.Close()

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	router := api.NewRouter(cfg, store.NewPostgres(pool), realtime.NewHub())
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("API listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}

// connectWithRetry keeps trying for ~30s so the API can start alongside a
// Postgres container that is still booting.
func connectWithRetry(ctx context.Context, databaseURL string) *pgxpool.Pool {
	var lastErr error
	for i := 0; i < 30; i++ {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool
			}
			pool.Close()
		}
		lastErr = err
		log.Printf("waiting for database... (%v)", err)
		select {
		case <-ctx.Done():
			log.Fatalf("interrupted while connecting to database: %v", lastErr)
		case <-time.After(time.Second):
		}
	}
	log.Fatalf("could not connect to database: %v", lastErr)
	return nil
}
